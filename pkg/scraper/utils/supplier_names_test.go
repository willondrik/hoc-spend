package utils

import "testing"

func TestNormalizeSupplierName(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "curated alias exact", raw: "Wal-Mart", want: "Walmart"},
		{name: "curated alias case insensitive", raw: "  walmart  ", want: "Walmart"},
		{name: "case and whitespace variants use curated canonical form", raw: "  aMAZON  ", want: "Amazon"},
		{name: "bilingual postal aliases", raw: "POSTES CANADA", want: "Canada Post / Postes Canada"},
		{name: "brand spelling alias", raw: "Globe and Mail", want: "The Globe and Mail"},
		{name: "truncated bilingual source alias", raw: "Toshiba Tec Canada Business Solutions – Toshiba Tec Canada Solut", want: "Toshiba Tec Canada Business Solutions / Toshiba Tec Canada Solutions d'affaires"},
		{name: "payee annotation", raw: "Abbotsford Chevon - Dennis A Semeniuk", want: "Abbotsford Chevon"},
		{name: "store number", raw: "Ace Hardware Air Ronge #71054", want: "Ace Hardware Air Ronge"},
		{name: "insurance legal suffix", raw: "Acera Insurance Services Ltd.", want: "Acera Insurance Services"},
		{name: "software legal suffix", raw: "Adobe", want: "Adobe Inc."},
		{name: "security legal suffix", raw: "ADT Security Services Canada, Inc.", want: "ADT Security Services Canada Inc."},
		{name: "contractor legal suffix", raw: "Alexander Security", want: "Alexander Security Inc."},
		{name: "linen service legal suffix", raw: "Alsco", want: "Alsco Canada Corporation"},
		{name: "amazon legal aliases", raw: "Amazon.Com.Ca,Inc.", want: "Amazon"},
		{name: "amazon web services remains distinct", raw: "Amazon Web Services", want: "Amazon Web Services"},
		{name: "generated punctuation alias", raw: "The Globe & Mail", want: "The Globe and Mail"},
		{name: "generated apostrophe alias", raw: "Festival D`Hiver De Roberval", want: "Festival D'Hiver De Roberval"},
		{name: "generated typography alias", raw: "Facebook/Meta", want: "Facebook - Meta"},
		{name: "generated French diacritic alias", raw: "Hydro-Québec", want: "Hydro Quebec"},
		{name: "generated business suffix alias", raw: "Zoom Communications Inc", want: "Zoom Communications, Inc."},
		{name: "blocked Fresh-Net merge remains distinct", raw: "Fresh-Net", want: "Fresh-Net"},
		{name: "blocked grocery banner merge remains distinct", raw: "Buy low Foods", want: "Buy Low Foods"},
		{name: "blocked delivery and restaurant composite remains distinct", raw: "Door Dash/Pizza Hut", want: "Door Dash/Pizza Hut"},
		{name: "blocked hotel mapping remains distinct", raw: "Fairmont Vancouver", want: "Fairmont Vancouver"},
		{name: "blocked general House of Commons mapping remains distinct", raw: "House of Commons / Chambre des communes", want: "House Of Commons / Chambre Des Communes"},
		{name: "blocked app intermediary mapping remains distinct", raw: "Apple Capcut", want: "Apple Capcut"},
		{name: "blocked typo target remains distinct", raw: "Lcbo/rap #0038", want: "Lcbo/Rap #0038"},
		{name: "unmapped mixed case is deterministic", raw: "NGP VAN Inc.", want: "NGP VAN Inc."},
		{name: "unmapped lower case matches mixed case", raw: "ngp van inc.", want: "NGP VAN Inc."},
		{name: "preserves common initialism", raw: "bc   hydro", want: "BC Hydro"},
		{name: "empty", raw: " \n\t ", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeSupplierName(test.raw); got != test.want {
				t.Fatalf("NormalizeSupplierName(%q) = %q, want %q", test.raw, got, test.want)
			}
		})
	}
}
