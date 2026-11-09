package utils

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// AUTO-GENERATED: Supplier name standardization mappings
// These mappings consolidate duplicate/variant supplier names to canonical forms
// Generated from name_audit.txt to fix incorrectly formatted supplier names

var SupplierNameCases = map[string]string{
	" Costco":                           "Costco",
	" Costco Wholesale":                 "Costco",
	" Dollarama":                        "Dollarama",
	" IGA":                              "IGA",
	" Real Canadian Superstore":         "Real Canadian Superstore",
	"7 Eleven":                          "7-Eleven",
	"A1 Cash and Carry":                 "A1 Cash & Carry",
	"AMAZON":                            "Amazon",
	"Amazon.com.ca ULC":                 "Amazon",
	"Bell Mobility – Bell Mobilité":     "Bell Mobility / Bell Mobilité",
	"Canada Post":                       "Canada Post / Postes Canada",
	"Canada Post - Postes Canada":       "Canada Post / Postes Canada",
	"Canada Post/Postes Canada":         "Canada Post / Postes Canada",
	"Globe and Mail":                    "The Globe and Mail",
	"MailChimp":                         "Mailchimp",
	"Meta Platforms Inc.":               "Meta Platforms, Inc.",
	"NGP Van Inc.":                      "NGP VAN Inc.",
	"NGP VAN Inc.":                      "NGP VAN Inc.",
	"Postes Canada":                     "Canada Post / Postes Canada",
	"Rogers Wireless – Rogers sans-fil": "Rogers Wireless / Rogers sans-fil",
	"Toshiba Tec Canada Business Solutions – Toshiba Tec Canada Solut":                "Toshiba Tec Canada Business Solutions / Toshiba Tec Canada Solutions d'affaires",
	"Toshiba Tec Canada Business Solutions – Toshiba Tec Canada Solutions d'affaires": "Toshiba Tec Canada Business Solutions / Toshiba Tec Canada Solutions d'affaires",
	"Amazon Canada":                       "Amazon",
	"Amazon.ca":                           "Amazon",
	"Bar Restaurant chez Mario Tremblay":  "Bar-Restaurant Mario Tremblay",
	"Biercraft Univercity":                "Biercraft",
	"Biercraft University":                "Biercraft",
	"Boucherie - Charcuterie La Bastille": "Charcuterie La Bastille",
	"Bronco's pub& Grill":                 "Bronco's Pub & Grill",
	"Buy low Foods":                       "Save-On-Foods",
	"CCITB":                               "Chambre de commerce et d'industrie Thérèse-De Blainville (CCITB)",
	"CCIVS - Chambre de commerce et d'industrie de Vaudreuil-Soulanges": "Chambre de Commerce et d'Industrie de Vaudreuil-Soulanges (CCIVS)",
	"CO-OP":                    "Co-Op",
	"COOP":                     "Co-Op",
	"COSTCO":                   "Costco",
	"COSTCO *542":              "Costco",
	"COSTCO WHOLESALE":         "Costco",
	"COSTCO Wholesale":         "Costco",
	"Cactus Club Cafe":         "Cactus Club",
	"Calgary Co-op":            "Co-Op",
	"Canadian Tire #026":       "Canadian Tire",
	"Canadian Tire #1049":      "Canadian Tire",
	"Canadian Tire #159":       "Canadian Tire",
	"Canadian Tire #188":       "Canadian Tire",
	"Canadian Tire #192":       "Canadian Tire",
	"Canadian Tire #351":       "Canadian Tire",
	"Canadian Tire 263":        "Canadian Tire",
	"Canadian Tire Dryden":     "Canadian Tire",
	"Canadian Tire St-Bruno":   "Canadian Tire",
	"Canadian Tire Store #047": "Canadian Tire",
	"Chalo FreshCo":            "FreshCo",
	"Chambre de Commerce et d'industrie Haute Yamaska":     "Chambre de Commerce et d'Industrie Haute-Yamaska",
	"Chambre de commerce Sherbrooke":                       "Chambre de Commerce Sherbrooke",
	"Chambre de commerce Val-d'Or":                         "Chambre de Commerce Val-d'Or",
	"Chambre de commerce Val-d`Or":                         "Chambre de Commerce Val-d'Or",
	"Chambre de commerce de l`industrie du Haut-Richelieu": "Chambre de Commerce de l`Industrie du Haut-Richelieu",
	"Chambre de commerce du Grand Joliette":                "Chambre de Commerce du Grand Joliette",
	"Chambre de commerce et d'industrie Grand Lévis":       "Chambre de Commerce et d'Industrie Grand Lévis",
	"Chambre de commerce et d'industrie de Mirabel":        "Chambre de Commerce et d'Industrie de Mirabel",
	"Chambre de commerce et de l'industrie Haute-Yamaska":  "Chambre de Commerce et d'Industrie Haute-Yamaska",
	"ChefBenKramer.com":                                    "Chef Ben Kramer Catering",
	"City Bread Co.LTD":                                    "City Bread Co.",
	"Club optimiste St-Janvier, Mirabel Inc.":              "Club Optimiste St-Janvier, Mirabel Inc.",
	"Co-Op (North Central)":                                "Co-Op",
	"Conception et production d'evenements Kimdja INC.":    "Conception et production d'événements Kimdja Inc.",
	"Conception et production d'événements Kimdja INC.":    "Conception et production d'événements Kimdja Inc.",
	"Concerts chapelle Saint-Cyriac":                       "Concerts Chapelle Saint-Cyriac",
	"Coop":                                                 "Co-Op",
	"Coopérative de solidarité le Relait":                  "Coopérative de solidarité Le Relait",
	"Costco Drumm ondville":                                "Costco",
	"Costco Dummondville":                                  "Costco",
	"Costco Gloucester 1362":                               "Costco",
	"Costco Wearhouse":                                     "Costco",
	"Costco Wholesale":                                     "Costco",
	"Costco Wholesale Vancouver":                           "Costco",
	"Couche-Tard 1300":                                     "Couchetard",
	"Couche-Tard GrandMère 868":                            "Couchetard",
	"CultureTix.ca":                                        "CultureTix",
	"DELTA HOTELS - MARRIOTT -BESSBOROUGH":                 "Delta Hotels",
	"DELTA HOTELS MARRIOTT BESSBOROUGH":                    "Delta Hotels",
	"Delta Hotel":                                          "Delta Hotels",
	"Delta Hotels Besssborough":                            "Delta Hotels",
	"Delta Hotels Marriot Bessborough":                     "Delta Hotels",
	"Depanneur du marché":                                  "Depanneur du Marché Inc",
	"Dollorama":                                            "Dollarama",
	"Dominos":                                              "Domino's Pizza",
	"Door Dash/Pizza Hut":                                  "Door Dash",
	"Dépanneur chez Rosie":                                 "Dépanneur Bière Experts",
	"Edmonton Hebrew Assoc.":                               "Edmonton Hebrew Association",
	"Fairmont Chateau Laurier":                             "Fairmont Hotel",
	"Fairmont Vancouver":                                   "Fiarmont Hotel",
	"Farine inc":                                           "Farine Inc",
	"Fire it Up":                                           "Fire It Up",
	"Food Services":                                        "Food Services House of Commons",
	"Food services":                                        "Food Services House of Commons",
	"Fresh Co":                                             "FreshCo",
	"Fresh-Net":                                            "FreshCo",
	"Freshco":                                              "FreshCo",
	"Freson Bros":                                          "Freson Brothers",
	"GARY'S NOFRILLS":                                      "NoFrills",
	"Gabe's Pizza":                                         "Gabriel Pizza",
	"Galati":                                               "Galati Market",
	"Gass Food Mart":                                       "Gass's Food Mart",
	"Highland Farms":                                       "Highland Farms Supermarket",
	"Hillman RE BC Liquor Store":                           "BC Liquor Store",
	"Hillman RE Superstore":                                "Superstore",
	"HoC Food Services Parliamentary Restaurant":           "Food Services House of Commons",
	"Horizon Thaïe":                                        "Horizon Thaï",
	"House Of Commons Food Services — Services De Restauration De La Chambre Des Communes": "Food Services House of Commons",
	"House of Commons / Chambre des communes":                                              "Food Services House of Commons",
	"House of Commons Food Services":                                                       "Food Services House of Commons",
	"House of Commons Food Services — Services de restauration de la Chambre des communes": "Food Services House of Commons",
	"IGA EXTRA FAMILLE BENOIT":                                                             "IGA",
	"IGA Extra Famille Benoit":                                                             "IGA",
	"IGA Grand-Mère":                                                                       "IGA",
	"Iga 8476":                                                                             "IGA",
	"King Kone Inc.":                                                                       "King Kone Inc",
	"La Boite a grains":                                                                    "La Boite à grains",
	"Lcbo/rap #0038":                                                                       "LBCO",
	"Lcbo/rap #538":                                                                        "LBCO",
	"Loblaws 1170":                                                                         "Loblaws",
	"MAXI":                                                                                 "Maxi",
	"MAxi":                                                                                 "Maxi",
	"MC DONALD":                                                                            "McDonald's",
	"METRO PLUS":                                                                           "Métro",
	"Magasin IGA Van Horne":                                                                "IGA",
	"Magasin Van Horne":                                                                    "IGA",
	"Metro 100244":                                                                         "Métro",
	"Metro Plus":                                                                           "Métro",
	"Metro Plus Bellemare":                                                                 "Métro",
	"Mitchan Sushi Inc.":                                                                   "Mitchan Sushi",
	"Métro plus":                                                                           "Métro",
	"NO FRILLS":                                                                            "NoFrills",
	"NOFRILLS":                                                                             "NoFrills",
	"Nemo sushi":                                                                           "Nemo Sushi",
	"Nespresso Canada":                                                                     "Nespresso",
	"No Frills":                                                                            "NoFrills",
	"No frills":                                                                            "NoFrills",
	"Nofrills":                                                                             "NoFrills",
	"Origine Cuisine Martime":                                                              "Origine Cuisine Maritime",
	"Origine Cuisine Martitime":                                                            "Origine Cuisine Maritime",
	"Osmows":                                                                               "Osmows Shwarma",
	"PARLIAMENTARY RESTAURANT":                                                             "Parliamentary Restaurant",
	"Petro-Canada":                                                                         "Petro Canada",
	"Philippine Heritage Council of Manitoba Inc.": "Philippine Heritage Council of Manitoba Inc",
	"Pizza pizza":                            "Pizza Pizza",
	"Poke A Bowl":                            "Poke a Bowl",
	"REAL CANADIAN SPERSTORE":                "Real Canadian Superstore",
	"REAL CANADIAN SUPERSTORE":               "Real Canadian Superstore",
	"Real Atlantic Superstore":               "Real Canadian Superstore",
	"Rebeltents":                             "Rebel Tents",
	"SAFEWAY":                                "Safeway",
	"SHOPPERS DRUG MART":                     "Shoppers Drug Mart",
	"SOBEY'S":                                "Sobey's",
	"SOBEYS":                                 "Sobey's",
	"Safeway Dryden":                         "Safeway",
	"Save On Foods":                          "Save-On-Foods",
	"Save On food":                           "Save-On-Foods",
	"Save on Foods":                          "Save-On-Foods",
	"Save-On-Food #946":                      "Save-On-Foods",
	"Save-on-Foods":                          "Save-On-Foods",
	"Save-on-food":                           "Save-On-Foods",
	"Services de restauration":               "Food Services House of Commons",
	"Services de restauration Ottawa":        "Food Services House of Commons",
	"Shopper Drug Mart":                      "Shoppers Drug Mart",
	"Shopper's Drug Mart":                    "Shoppers Drug Mart",
	"Shopper's Drugmart":                     "Shoppers Drug Mart",
	"Shoppers Drugmart":                      "Shoppers Drug Mart",
	"Sobeys":                                 "Sobey's",
	"Tim Horton's":                           "Tim Hortons",
	"Wal-Mart":                               "Walmart",
	"Wal-Mart Canada":                        "Walmart",
	"Walmart Canada":                         "Walmart",
	"Walmart Store 3024":                     "Walmart",
	"Walmart.ca":                             "Walmart",
	"Warlmart":                               "Walmart",
	"canadian tire":                          "Canadian Tire",
	"costco":                                 "Costco",
	"cro magnon":                             "CRO MAGNON",
	"cro-magnon":                             "CRO MAGNON",
	"delta hotels":                           "Delta Hotels",
	"dollarama":                              "Dollarama",
	"metro":                                  "Métro",
	"nespresso":                              "Nespresso",
	"petro Canada":                           "Petro Canada",
	"petro-canada":                           "Petro Canada",
	"save-on-food":                           "Save-On-Foods",
	"save-on-foods":                          "Save-On-Foods",
	"shoopers drug mart":                     "Shoppers Drug Mart",
	"tim hortons":                            "Tim Hortons",
	"walmart":                                "Walmart",
	"1-800 GOT JUNK?":                        "1-800-GOT-JUNK",
	"1-800-GOT-JUNK?":                        "1-800-GOT-JUNK",
	"0744357 BC Ltd - Diana'S Monograming":   "0744357 BC Ltd. - Diana's Monogramming",
	"0744357 BC Ltd. - Diana'S Monogramming": "0744357 BC Ltd. - Diana's Monogramming",
	"4imprint":                               "4 Imprint",
	"4Imprint":                               "4 Imprint",
	"4 imprint CANADA":                       "4 Imprint",
	"4imprint, Inc":                          "4 Imprint",
	"4IMPRINT, INC.":                         "4 Imprint",
	"a1cashandcarry.com":                     "A1 Cash and Carry",
	"A1CashandCarry":                         " A1 Cash and Carry",
	"Abbotsford Chevon - Dennis A Semeniuk":  "Abbotsford Chevon",
	"Ace Hardware Air Ronge":                 "Ace Hardware Air Ronge",
	"Ace Hardware Air Ronge #71054":          "Ace Hardware Air Ronge",
	"Acera":                                  "Acera Insurance Services",
	"Acera Insurance":                        "Acera Insurance Services",
	"Acera Insurance Broker Network":         "Acera Insurance Services",
	"Acera Insurance Services":               "Acera Insurance Services",
	"Acera Insurance Services Ltd":           "Acera Insurance Services",
	"Acera Insurance Services Ltd.":          "Acera Insurance Services",
	"Adobe":                                  "Adobe Inc.",
	"Adobe Inc":                              "Adobe Inc.",
	"Adobe Inc.":                             "Adobe Inc.",
	"Adt":                                    "ADT Security Services Canada Inc.",
	"Adt Security":                           "ADT Security Services Canada Inc.",
	"Adt Security Services Canada Inc.":      "ADT Security Services Canada Inc.",
	"Adt Security Services Canada, Inc.":     "ADT Security Services Canada Inc.",
	"Alexander Security":                     "Alexander Security Inc.",
	"Alexander Security Inc.":                "Alexander Security Inc.",
	"Alpha Integrated Systems Ltd":           "Alpha Integrated Systems Ltd.",
	"Alpha Integrated Systems Ltd.":          "Alpha Integrated Systems Ltd.",
	"Alsco":                                  "Alsco Canada Corporation",
	"Alsco Canada":                           "Alsco Canada Corporation",
	"Alsco Canada Corporation":               "Alsco Canada Corporation",
	"Amazon.":                                "Amazon",
	"Amazon Ca":                              "Amazon",
	"Amazon.Ca.Ca ULC":                       "Amazon",
	"Amazon.Ca ULC":                          "Amazon",
	"Amazon.Com":                             "Amazon",
	"Amazon.Com.Ca":                          "Amazon",
	"Amazon.Com.Ca,Inc.":                     "Amazon",
	"Amazon.Com Inc.":                        "Amazon",
	"Amazon.Com, Inc.":                       "Amazon",
}

var supplierNameFoldedCases = buildSupplierNameFoldedCases()

// NormalizeSupplierName returns a deterministic supplier representation. It
// first applies the curated aliases above, including aliases whose source value
// differs only in case or whitespace. Unmapped values receive stable casing so
// source pages that randomly capitalize a vendor differently do not create
// separate suppliers on a fresh scrape.
func NormalizeSupplierName(name string) string {
	name = collapseSupplierWhitespace(name)
	if name == "" {
		return ""
	}
	if normalized, exists := SupplierNameCases[name]; exists {
		return collapseSupplierWhitespace(normalized)
	}
	if normalized, exists := supplierNameFoldedCases[supplierNameFoldKey(name)]; exists {
		return normalized
	}
	return canonicalSupplierCasing(name)
}

func buildSupplierNameFoldedCases() map[string]string {
	keys := make([]string, 0, len(SupplierNameCases))
	for source := range SupplierNameCases {
		keys = append(keys, source)
	}
	sort.Strings(keys)

	result := make(map[string]string, len(SupplierNameCases)*2)
	for _, source := range keys {
		canonical := collapseSupplierWhitespace(SupplierNameCases[source])
		key := supplierNameFoldKey(source)
		if existing, found := result[key]; !found || canonical < existing {
			result[key] = canonical
		}
		canonicalKey := supplierNameFoldKey(canonical)
		if existing, found := result[canonicalKey]; !found || canonical < existing {
			result[canonicalKey] = canonical
		}
	}
	return result
}

func supplierNameFoldKey(name string) string {
	return strings.ToLower(collapseSupplierWhitespace(name))
}

func collapseSupplierWhitespace(name string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
}

func canonicalSupplierCasing(name string) string {
	var result strings.Builder
	for _, token := range strings.Fields(strings.ToLower(name)) {
		if result.Len() > 0 {
			result.WriteByte(' ')
		}
		result.WriteString(canonicalSupplierToken(token))
	}
	return result.String()
}

func canonicalSupplierToken(token string) string {
	var result strings.Builder
	for start := 0; start < len(token); {
		end := start
		for end < len(token) {
			r, size := utf8.DecodeRuneInString(token[end:])
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				break
			}
			end += size
		}
		if end == start {
			_, size := utf8.DecodeRuneInString(token[start:])
			result.WriteString(token[start : start+size])
			start += size
			continue
		}
		result.WriteString(canonicalSupplierWord(token[start:end]))
		start = end
	}
	return result.String()
}

func canonicalSupplierWord(word string) string {
	if word == "" {
		return ""
	}
	if supplierInitialisms[word] {
		return strings.ToUpper(word)
	}
	runes := []rune(word)
	for i, r := range runes {
		if unicode.IsLetter(r) {
			runes[i] = unicode.ToUpper(r)
			break
		}
	}
	return string(runes)
}

// The fallback intentionally keeps this list small. Curated aliases remain the
// place for vendor-specific branding; these are unambiguous initialisms found
// commonly in supplier data.
var supplierInitialisms = map[string]bool{
	"bc": true, "bmo": true, "cibc": true, "g": true, "gp": true,
	"h2o": true, "iga": true, "lp": true, "ngp": true, "rbc": true,
	"td": true, "ulc": true,
}
