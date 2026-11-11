package transform

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/willondrik/hoc-spend/pkg/scraper/types"
)

func removeHonorifics(s string) string {
	s = strings.TrimSpace(s)
	for _, honorific := range []string{"The Right Hon.", "Right Hon.", "The Hon.", "Hon.", "Honourable", "Honorable"} {
		if len(s) > len(honorific) && strings.EqualFold(s[:len(honorific)], honorific) && unicode.IsSpace(rune(s[len(honorific)])) {
			return strings.TrimSpace(s[len(honorific):])
		}
	}
	return s
}

// parseCommaName parses names of the form "Last, First".
//
// It also supports a 3-part comma form observed in officer travel pages
// (e.g. "Last Part1, Last Part2, First").
func parseCommaName(name string) types.PersonName {
	name = removeHonorifics(name)
	parts := strings.Split(name, ", ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	if len(parts) == 3 {
		return types.PersonName{FirstName: parts[2], LastName: strings.TrimSpace(parts[0] + " " + parts[1])}
	}
	if len(parts) < 2 {
		return types.PersonName{}
	}
	return types.PersonName{FirstName: parts[1], LastName: parts[0]}
}

// parseTravellerName supports entries like:
//
//	"Type (Last, First)"
//	"Last, First"
//
// and strips honorifics.
func parseTravellerName(raw string) (types.PersonName, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return types.PersonName{}, nil
	}

	// Edge cases observed in data:
	//   "Walsh, Liam, Liam"
	//   "Nath, Arjun, Arjun"
	if strings.Count(raw, ",") >= 2 {
		lastComma := strings.LastIndex(raw, ",")
		if lastComma == -1 {
			return types.PersonName{}, fmt.Errorf("unexpected traveller name: %s", raw)
		}
		raw = strings.TrimSpace(raw[:lastComma])
	}

	if strings.Contains(raw, "(") && strings.Contains(raw, ")") {
		open := strings.IndexByte(raw, '(')
		close := strings.LastIndexByte(raw, ')')
		if open == -1 || close == -1 || close <= open {
			return types.PersonName{}, fmt.Errorf("invalid traveller name: %s", raw)
		}
		inner := strings.TrimSpace(raw[open+1 : close])
		return parseCommaName(inner), nil
	}

	return parseCommaName(raw), nil
}

func parsePlacesVisited(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			v := strings.TrimSpace(p)
			if v != "" {
				out = append(out, v)
			}
		}
		return out
	}
	return []string{s}
}

/*
func normalizeAdminContractDescription(desc string) (string, error) {
	formatMap := map[string]string{
		"Banking Fees and Cheque orders":                                     "Banking Fees and Cheque Orders",
		"Cable and internet fees":                                            "Cable and Internet",
		"Cable and Internet Fees":                                            "Cable and Internet",
		"Internet and Cable":                                                 "Cable and Internet",
		"Cameras and audio/visual capture equipment":                         "Cameras and Audio/Visual Capture Equipment",
		"Changes to voice communication services and equipment":              "Changes to Voice Communication Services and Equipment",
		"Cleaning products":                                                  "Cleaning Products",
		"Client Software Related to Distributed Computing Environment (DCE)": "Client Software Related to Distributed Computing Environment",
		"Client Software related to Distributed Computing Environment (including initial cost such as licensing fees)": "Client Software Related to Distributed Computing Environment",
		"Computer Equipment – Large/Medium – Mainframe/Mini/Servers":                                                   "Computer Equipment",
		"Computer Equipment – Small – Desktop/Personal/Portable/Keyboard":                                              "Computer Equipment",
		"Constituency Office Rent and Related Fees":                                                                    "Constituency Office Rental and Related Fees",
		"Consultants (Member's Office Budget)":                                                                         "Consultants",
		"Data Communications Services (Charges for all leased non-voice telecommunications services, e.g. Ethernet)":   "Data Communications Services",
		"Data Communication Services (Charges for all leased non-voice telecommunications services)":                   "Data Communications Services",
		"Gifts given as a matter of protocol":                                                                          "Gifts (given as a matter of protocol)",
		"Householders - Paper Recovery":                                                                                "Householders",
		"Image/Video Communication Services (e.g. Zoom licence fees)":                                                  "Image/Video Communcation Services",
		"Image/Video communications services":                                                                          "Image/Video Communcation Services",
		"Information Technology (IT) Support Officer":                                                                  "IT Support Officer",
		"F181 - IT Support Officer":                                                                                    "IT Support Officer",
		"Insurance Premiums - vehicle":                                                                                 "Insurance Premiums",
		"Long Distance - Federal Government Departments":                                                               "Long Distance",
		"Monthly Phone Service - Federal Government Departments":                                                       "Monthly Phone Service",
		"Monthly Telephone Service":                                                                                    "Monthly Phone Service",
		" Monthly Telephone Service - (landlines)":                                                                     "Monthly Phone Service",
		"Monthly Telephone Service (landlines) – OGD":                                                                  "Monthly Phone Service",
		"Office Equipment (Calculators, Headsets, etc.)":                                                               "Office Equipment",
		"Office furniture and fixtures":                                                                                "Office Furniture",
		"Office Furniture – Bookcases, Cabinets, Chairs, Sofas":                                                        "Office Furniture",
		"Office Rental":                                                   "Office Rental and Fees",
		"Office Rental and Related Fees":                                  "Office Rental and Fees",
		"Office Rental Related Fees":                                      "Office Rental and Fees",
		"Office Supplies, Stationery, and Accessories":                    "Office Supplies",
		"Other Rentals- Federal Government Departments":                   "Office Rental and Fees",
		"Other Services - Federal Government Deparments":                  "Other Services",
		"Other Telecommunications Services":                               "Other Telecommunication Services",
		"Photocopier Equipment (Copy Charge)":                             "Photocopier Equipment",
		"Photography and Video Services - Federal Government Departments": "Photography and Video Services",
		"Photography and Video Services, Including Film Proc":             "Photography and Video Services",
		"Postage":                 "Postage, Parcel Post and Courier Services",
		"Postage and Parcel Post": "Postage, Parcel Post and Courier Services",
		"Printed Material, Books, Newspapers and Subscriptions":                                                                    "Printed Material",
		"Printed Matter, Books, Newspapers and Subscriptions":                                                                      "Printed Material",
		"Printed Matter - Federal Government Departments":                                                                          "Printed Material",
		"Printed Matter such as: Printed Envelopes, Maps, Printed Forms, Transparencies, Labels":                                   "Printed Material",
		"Professional Communication Services such as Speech Writing, Text Editing, Art Work, Media Relations and Media Monitoring": "Professional Communcation Services",
		"Protection and Alarm Monitoring (Member's Office Budget)":                                                                 "Protection and Alarm Monitoring",
		"Protection and Alarm Monitoring (MOB)":                                                                                    "Protection and Alarm Monitoring",
		"Public Meeting Rooms":                                                                                                     "Public Meeting Room Rentals",
		"Repair of Heating, Air-conditioning and Refrigeration Equipment":                                                          "Repair of Heating, Air Conditioning and Refrigeration Equipment",
		"Repair of Telecommuncations Equipment":                                                                                    "Repair of Telecommunication Equipment",
		"Repairs - Office Equipment and Furniture":                                                                                 "Repairs of Office Equipment and Furniture",
		"Research Consultants - Federal Government Departments":                                                                    "Research Consultants",
		"Security Equipment, intercom and Alarm System":                                                                            "Security Equipment and Services",
		"Security Equipment, Intercom and Alarm System (Central Budget)":                                                           "Security Equipment and Services",
		"Security Equipment, Intercom and Alarm System (Member's Office Budget)":                                                   "Security Equipment and Services",
		"Third party advertisement":                                                                                                "Third Party Advertisement",
		" Toll-free - Charges and Calls - Federal Government Departments":                                                          "Toll-Free Charges and Calls",
		"Toll-free - Charges and Calls - OGD":                                                                                      "Toll-Free Charges and Calls",

		"Training Programs / Courses - Federal Government Departments (off site)": "Training Programs and Courses",
		"Training Programs / Courses (off site)":                                  "Training Programs and Courses",
		"Training Programs / Courses (on site)":                                   "Training Programs and Courses",
		"Translation and Transcription Services":                                  "Translation Services",

		"Translation Services - Federal Government Departments":       "Translation Services",
		"Video Communication, Broadcasting Equipment and Televisions": "Video Communcation and Broadcasting Equipment",
	}
}
*/
