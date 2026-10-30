package ledger

import "testing"

func TestTransactionValidate(t *testing.T) {
	cases := []struct {
		name string
		tx   Transaction
		want bool
	}{
		{
			name: "valid",
			tx:   Transaction{Amount: 100, Category: "еда"},
			want: false,
		},
		{
			name: "zero amount",
			tx:   Transaction{Amount: 0, Category: "еда"},
			want: true,
		},
		{
			name: "negative amount",
			tx:   Transaction{Amount: -1, Category: "еда"},
			want: true,
		},
		{
			name: "empty category",
			tx:   Transaction{Amount: 100},
			want: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.tx.Validate()
			if (err != nil) != tc.want {
				t.Fatalf("Transaction.Validate() error = %v, want error: %v", err, tc.want)
			}
		})
	}
}

func TestBudgetValidate(t *testing.T) {
	cases := []struct {
		name   string
		budget Budget
		want   bool
	}{
		{
			name:   "positive limit",
			budget: Budget{Category: "еда", Limit: 5000},
			want:   false,
		},
		{
			name:   "zero limit",
			budget: Budget{Category: "еда", Limit: 0},
			want:   true,
		},
		{
			name:   "negative limit",
			budget: Budget{Category: "еда", Limit: -1},
			want:   true,
		},
		{
			name:   "empty category",
			budget: Budget{Limit: 5000},
			want:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.budget.Validate()
			if (err != nil) != tc.want {
				t.Fatalf("Budget.Validate() error = %v, want error: %v", err, tc.want)
			}
		})
	}
}
