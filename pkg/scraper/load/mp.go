package load

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

// MPProfileLoadResult describes the outcome of an additive MP profile load.
type MPProfileLoadResult struct {
	Received       int
	DuplicateInput int
	Existing       int
	Inserted       int
}

// InsertNewMPProfiles writes only profiles whose non-empty ProfileURL is not
// already stored in mps.profile_url. Profile URL is used deliberately instead
// of name, riding, or caucus because those values are not durable MP identity
// keys and may change over a parliamentary career.
func (l *Loader) InsertNewMPProfiles(ctx context.Context, profiles []*types.MPDetails) (MPProfileLoadResult, error) {
	result := MPProfileLoadResult{Received: len(profiles)}
	unique, duplicates, err := uniqueMPProfiles(profiles)
	if err != nil {
		return result, err
	}
	result.DuplicateInput = duplicates
	if len(unique) == 0 {
		return result, nil
	}

	existing, err := l.existingMPProfileURLs(ctx, unique)
	if err != nil {
		return result, err
	}
	result.Existing = len(existing)

	newProfiles := filterNewMPProfiles(unique, existing)
	if len(newProfiles) == 0 {
		return result, nil
	}

	tx, err := l.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("load MP profiles: begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // A committed transaction cannot be rolled back.

	batch := &pgx.Batch{}
	for _, profile := range newProfiles {
		hillOffice, err := json.Marshal(profile.Contact.HillOffice)
		if err != nil {
			return result, fmt.Errorf("load MP profile %q: marshal hill office: %w", profile.ProfileURL, err)
		}
		constituencyOffice, err := json.Marshal(profile.Contact.ConstituencyOffice)
		if err != nil {
			return result, fmt.Errorf("load MP profile %q: marshal constituency office: %w", profile.ProfileURL, err)
		}
		roles := profile.Roles
		if roles == nil {
			roles = []string{}
		}
		rolesJSON, err := json.Marshal(roles)
		if err != nil {
			return result, fmt.Errorf("load MP profile %q: marshal roles: %w", profile.ProfileURL, err)
		}

		// The advisory lock serializes concurrent loads of the same profile URL.
		// It prevents an otherwise possible orphan people row when a competing
		// transaction wins the mps.profile_url unique-key race.
		batch.Queue(`
WITH profile_lock AS (
    SELECT pg_advisory_xact_lock(hashtextextended($5, 0))
),
existing_mp AS (
    SELECT id
    FROM mps, profile_lock
    WHERE profile_url = $5
),
inserted_person AS (
    INSERT INTO people (first_name, last_name)
    SELECT $1, $2
    WHERE NOT EXISTS (SELECT 1 FROM existing_mp)
    RETURNING id
)
INSERT INTO mps (
    person_id, province, profile_url, avatar, email, website,
    hill_office, constituency_office, roles
)
SELECT id, $3, $5, $4, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb
FROM inserted_person
ON CONFLICT (profile_url) DO NOTHING`,
			profile.Name.FirstName,
			profile.Name.LastName,
			strings.TrimSpace(profile.Province),
			profile.Avatar,
			profile.ProfileURL,
			strings.TrimSpace(profile.Contact.Email),
			strings.TrimSpace(profile.Contact.Website),
			string(hillOffice),
			string(constituencyOffice),
			string(rolesJSON),
		)
	}

	batchResults := tx.SendBatch(ctx, batch)
	for range newProfiles {
		tag, err := batchResults.Exec()
		if err != nil {
			batchResults.Close()
			return result, fmt.Errorf("load MP profiles: insert profile: %w", err)
		}
		result.Inserted += int(tag.RowsAffected())
	}
	if err := batchResults.Close(); err != nil {
		return result, fmt.Errorf("load MP profiles: close insert batch: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("load MP profiles: commit transaction: %w", err)
	}

	return result, nil
}

func (l *Loader) existingMPProfileURLs(ctx context.Context, profiles []*types.MPDetails) (map[string]struct{}, error) {
	urls := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		urls = append(urls, profile.ProfileURL)
	}

	rows, err := l.pool.Query(ctx, `
SELECT profile_url
FROM mps
WHERE profile_url = ANY($1::text[])`, urls)
	if err != nil {
		return nil, fmt.Errorf("load MP profiles: query existing profile URLs: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]struct{})
	for rows.Next() {
		var profileURL string
		if err := rows.Scan(&profileURL); err != nil {
			return nil, fmt.Errorf("load MP profiles: scan existing profile URL: %w", err)
		}
		existing[profileURL] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load MP profiles: read existing profile URLs: %w", err)
	}

	return existing, nil
}

func uniqueMPProfiles(profiles []*types.MPDetails) ([]*types.MPDetails, int, error) {
	unique := make([]*types.MPDetails, 0, len(profiles))
	seen := make(map[string]struct{}, len(profiles))
	duplicates := 0

	for index, profile := range profiles {
		if profile == nil {
			return nil, duplicates, fmt.Errorf("load MP profiles: profile at index %d is nil", index)
		}
		profile.ProfileURL = strings.TrimSpace(profile.ProfileURL)
		profile.Name.FirstName = strings.TrimSpace(profile.Name.FirstName)
		profile.Name.LastName = strings.TrimSpace(profile.Name.LastName)
		if profile.ProfileURL == "" {
			return nil, duplicates, fmt.Errorf("load MP profiles: profile at index %d has an empty profile URL", index)
		}
		if profile.Name.FirstName == "" && profile.Name.LastName == "" {
			return nil, duplicates, fmt.Errorf("load MP profiles: profile %q has an empty name", profile.ProfileURL)
		}
		if _, exists := seen[profile.ProfileURL]; exists {
			duplicates++
			continue
		}
		seen[profile.ProfileURL] = struct{}{}
		unique = append(unique, profile)
	}
	// A deterministic lock order avoids deadlocks when concurrent scraper jobs
	// contain overlapping profile batches.
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].ProfileURL < unique[j].ProfileURL
	})

	return unique, duplicates, nil
}

func filterNewMPProfiles(profiles []*types.MPDetails, existing map[string]struct{}) []*types.MPDetails {
	newProfiles := make([]*types.MPDetails, 0, len(profiles))
	for _, profile := range profiles {
		if _, found := existing[profile.ProfileURL]; !found {
			newProfiles = append(newProfiles, profile)
		}
	}
	return newProfiles
}
