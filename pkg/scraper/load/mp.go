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

// MPProfileEnrichmentResult reports a conservative update from the current
// House member directory to a disclosure-backed MP identity.
type MPProfileEnrichmentResult struct {
	Updated   bool
	Unmatched bool
	Ambiguous bool
}

// EnrichMPProfile updates a disclosure-backed MP with current House profile
// data only when its normalized name and constituency identify exactly one MP.
// It writes the canonical public House URL to mps.profile_url while retaining
// mps.source_identity as the internal historical disclosure key.
func (l *Loader) EnrichMPProfile(ctx context.Context, profile *types.MPDetails, constituency string) (MPProfileEnrichmentResult, error) {
	result := MPProfileEnrichmentResult{}
	if profile == nil {
		return result, fmt.Errorf("enrich MP profile: profile is nil")
	}
	firstName := strings.TrimSpace(profile.Name.FirstName)
	lastName := strings.TrimSpace(profile.Name.LastName)
	constituency = strings.TrimSpace(constituency)
	profileURL := strings.TrimSpace(profile.ProfileURL)
	if (firstName == "" && lastName == "") || constituency == "" || profileURL == "" {
		return result, fmt.Errorf("enrich MP profile: name, constituency, and public profile URL are required")
	}

	rows, err := l.pool.Query(ctx, `
SELECT mps.id
FROM mps
JOIN people ON people.id = mps.person_id
JOIN mp_affiliations ON mp_affiliations.mp_id = mps.id
WHERE lower(btrim(people.first_name)) = lower($1)
  AND lower(btrim(people.last_name)) = lower($2)
  AND lower(btrim(mp_affiliations.constituency)) = lower($3)
GROUP BY mps.id
ORDER BY mps.id
LIMIT 2`, firstName, lastName, constituency)
	if err != nil {
		return result, fmt.Errorf("enrich MP profile: find disclosure identity: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return result, fmt.Errorf("enrich MP profile: scan disclosure identity: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("enrich MP profile: read disclosure identities: %w", err)
	}
	if len(ids) == 0 {
		result.Unmatched = true
		return result, nil
	}
	if len(ids) > 1 {
		result.Ambiguous = true
		return result, nil
	}

	hillOffice, err := json.Marshal(profile.Contact.HillOffice)
	if err != nil {
		return result, fmt.Errorf("enrich MP profile: marshal hill office: %w", err)
	}
	constituencyOffice, err := json.Marshal(profile.Contact.ConstituencyOffice)
	if err != nil {
		return result, fmt.Errorf("enrich MP profile: marshal constituency office: %w", err)
	}
	roles := profile.Roles
	if roles == nil {
		roles = []string{}
	}
	rolesJSON, err := json.Marshal(roles)
	if err != nil {
		return result, fmt.Errorf("enrich MP profile: marshal roles: %w", err)
	}

	if _, err := l.pool.Exec(ctx, `
UPDATE mps
SET province = $2,
    profile_url = $3,
    avatar = $4,
    email = $5,
    website = $6,
    hill_office = $7::jsonb,
    constituency_office = $8::jsonb,
    roles = $9::jsonb,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1`, ids[0], strings.TrimSpace(profile.Province), profileURL, profile.Avatar,
		strings.TrimSpace(profile.Contact.Email), strings.TrimSpace(profile.Contact.Website),
		string(hillOffice), string(constituencyOffice), string(rolesJSON)); err != nil {
		return result, fmt.Errorf("enrich MP profile: update profile: %w", err)
	}
	result.Updated = true
	return result, nil
}

// InsertNewMPProfiles writes only profiles whose non-empty SourceIdentity is
// not already stored in mps.source_identity. The synthetic source identity is
// separate from the optional current public House profile URL.
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

	existing, err := l.existingMPSourceIdentities(ctx, unique)
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
			return result, fmt.Errorf("load MP source identity %q: marshal hill office: %w", profile.SourceIdentity, err)
		}
		constituencyOffice, err := json.Marshal(profile.Contact.ConstituencyOffice)
		if err != nil {
			return result, fmt.Errorf("load MP source identity %q: marshal constituency office: %w", profile.SourceIdentity, err)
		}
		roles := profile.Roles
		if roles == nil {
			roles = []string{}
		}
		rolesJSON, err := json.Marshal(roles)
		if err != nil {
			return result, fmt.Errorf("load MP source identity %q: marshal roles: %w", profile.SourceIdentity, err)
		}

		// The advisory lock serializes concurrent loads of the same source identity.
		// It prevents an otherwise possible orphan people row when a competing
		// transaction wins the mps.source_identity unique-key race.
		batch.Queue(`
WITH profile_lock AS (
    SELECT pg_advisory_xact_lock(hashtextextended($5, 0))
),
existing_mp AS (
    SELECT id
    FROM mps, profile_lock
    WHERE source_identity = $5
),
inserted_person AS (
    INSERT INTO people (first_name, last_name)
    SELECT $1, $2
    WHERE NOT EXISTS (SELECT 1 FROM existing_mp)
    RETURNING id
)
INSERT INTO mps (
    person_id, province, profile_url, source_identity, avatar, email, website,
    hill_office, constituency_office, roles
)
SELECT id, $3, NULL, $5, $4, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb
FROM inserted_person
ON CONFLICT (source_identity) DO NOTHING`,
			profile.Name.FirstName,
			profile.Name.LastName,
			strings.TrimSpace(profile.Province),
			profile.Avatar,
			profile.SourceIdentity,
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

func (l *Loader) existingMPSourceIdentities(ctx context.Context, profiles []*types.MPDetails) (map[string]struct{}, error) {
	identities := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		identities = append(identities, profile.SourceIdentity)
	}

	rows, err := l.pool.Query(ctx, `
SELECT source_identity
FROM mps
WHERE source_identity = ANY($1::text[])`, identities)
	if err != nil {
		return nil, fmt.Errorf("load MP profiles: query existing source identities: %w", err)
	}
	defer rows.Close()

	existing := make(map[string]struct{})
	for rows.Next() {
		var sourceIdentity string
		if err := rows.Scan(&sourceIdentity); err != nil {
			return nil, fmt.Errorf("load MP profiles: scan existing source identity: %w", err)
		}
		existing[sourceIdentity] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load MP profiles: read existing source identities: %w", err)
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
		profile.SourceIdentity = strings.TrimSpace(profile.SourceIdentity)
		profile.Name.FirstName = strings.TrimSpace(profile.Name.FirstName)
		profile.Name.LastName = strings.TrimSpace(profile.Name.LastName)
		if profile.SourceIdentity == "" {
			return nil, duplicates, fmt.Errorf("load MP profiles: profile at index %d has an empty source identity", index)
		}
		if profile.Name.FirstName == "" && profile.Name.LastName == "" {
			return nil, duplicates, fmt.Errorf("load MP profiles: source identity %q has an empty name", profile.SourceIdentity)
		}
		if _, exists := seen[profile.SourceIdentity]; exists {
			duplicates++
			continue
		}
		seen[profile.SourceIdentity] = struct{}{}
		unique = append(unique, profile)
	}
	// A deterministic lock order avoids deadlocks when concurrent scraper jobs
	// contain overlapping profile batches.
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].SourceIdentity < unique[j].SourceIdentity
	})

	return unique, duplicates, nil
}

func filterNewMPProfiles(profiles []*types.MPDetails, existing map[string]struct{}) []*types.MPDetails {
	newProfiles := make([]*types.MPDetails, 0, len(profiles))
	for _, profile := range profiles {
		if _, found := existing[profile.SourceIdentity]; !found {
			newProfiles = append(newProfiles, profile)
		}
	}
	return newProfiles
}
