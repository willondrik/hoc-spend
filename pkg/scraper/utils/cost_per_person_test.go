package utils

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCostPerPerson(t *testing.T) {
	tests := []struct {
		name       string
		total      string
		attendance int
		want       string
	}{
		{name: "rounds to cents", total: "100", attendance: 3, want: "33.33"},
		{name: "keeps refund sign", total: "-12.50", attendance: 2, want: "-6.25"},
		{name: "empty attendance returns zero", total: "100", attendance: 0, want: "0"},
		{name: "negative attendance returns zero", total: "100", attendance: -1, want: "0"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			total, err := decimal.NewFromString(test.total)
			if err != nil {
				t.Fatalf("parse total: %v", err)
			}
			want, err := decimal.NewFromString(test.want)
			if err != nil {
				t.Fatalf("parse expected result: %v", err)
			}

			if got := CostPerPerson(total, test.attendance); !got.Equal(want) {
				t.Fatalf("CostPerPerson(%s, %d) = %s; want %s", total, test.attendance, got, want)
			}
		})
	}
}
