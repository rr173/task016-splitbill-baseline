package splitbill

import (
	"errors"
	"testing"
)

func ptr(i int64) *int64 { return &i }

// TestEqualSplitRounding 验证等额分摊的补差：1000 分给 3 人，首位多 1 分，之和精确等于总额。
func TestEqualSplitRounding(t *testing.T) {
	shares, err := ComputeShares(1000, ModeEqual, []ParticipantInput{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int64{334, 333, 333}
	var sum int64
	for i, s := range shares {
		if s.AmountCents != want[i] {
			t.Fatalf("share %d = %d, want %d", i, s.AmountCents, want[i])
		}
		sum += s.AmountCents
	}
	if sum != 1000 {
		t.Fatalf("sum = %d, want 1000", sum)
	}
}

// TestEqualSplitSmallAmount 金额小于人数：2 分给 3 人得到 [1,1,0]。
func TestEqualSplitSmallAmount(t *testing.T) {
	shares, _ := ComputeShares(2, ModeEqual, []ParticipantInput{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	})
	want := []int64{1, 1, 0}
	for i, s := range shares {
		if s.AmountCents != want[i] {
			t.Fatalf("share %d = %d, want %d", i, s.AmountCents, want[i])
		}
	}
}

// TestRatioSplitRounding 按比例分摊补差：101 分按权重 [1,2,1] 得 [26,50,25]。
func TestRatioSplitRounding(t *testing.T) {
	shares, err := ComputeShares(101, ModeRatio, []ParticipantInput{
		{Name: "a", Weight: 1}, {Name: "b", Weight: 2}, {Name: "c", Weight: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int64{26, 50, 25}
	var sum int64
	for i, s := range shares {
		if s.AmountCents != want[i] {
			t.Fatalf("share %d = %d, want %d", i, s.AmountCents, want[i])
		}
		sum += s.AmountCents
	}
	if sum != 101 {
		t.Fatalf("sum = %d, want 101", sum)
	}
}

// TestRatioZeroWeightExcludedFromRemainder 权重为 0 者份额为 0 且不参与补差。
// 101 分按权重 [0,1,0,1] 得 [0,51,0,50]：补差只给权重大于 0 的首位 b。
func TestRatioZeroWeightExcludedFromRemainder(t *testing.T) {
	shares, err := ComputeShares(101, ModeRatio, []ParticipantInput{
		{Name: "a", Weight: 0}, {Name: "b", Weight: 1}, {Name: "c", Weight: 0}, {Name: "d", Weight: 1},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []int64{0, 51, 0, 50}
	var sum int64
	for i, s := range shares {
		if s.AmountCents != want[i] {
			t.Fatalf("share %d = %d, want %d", i, s.AmountCents, want[i])
		}
		sum += s.AmountCents
	}
	if sum != 101 {
		t.Fatalf("sum = %d, want 101", sum)
	}
}

// TestRatioAllZeroWeightsRejected 全部权重为 0 必须拒绝。
func TestRatioAllZeroWeightsRejected(t *testing.T) {
	_, err := ComputeShares(100, ModeRatio, []ParticipantInput{
		{Name: "a", Weight: 0}, {Name: "b", Weight: 0},
	})
	if !errors.Is(err, ErrRatioWeightsInvalid) {
		t.Fatalf("want ErrRatioWeightsInvalid, got %v", err)
	}
}

// TestRatioNegativeWeightRejected 负权重必须拒绝。
func TestRatioNegativeWeightRejected(t *testing.T) {
	_, err := ComputeShares(100, ModeRatio, []ParticipantInput{
		{Name: "a", Weight: -1}, {Name: "b", Weight: 2},
	})
	if !errors.Is(err, ErrRatioWeightsInvalid) {
		t.Fatalf("want ErrRatioWeightsInvalid, got %v", err)
	}
}

// TestFixedExactSum 固定额之和恰好等于总额：自由参与者份额为 0。
func TestFixedExactSum(t *testing.T) {
	shares, err := ComputeShares(1000, ModeFixed, []ParticipantInput{
		{Name: "a", Fixed: ptr(400)}, {Name: "b", Fixed: ptr(600)}, {Name: "c"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]int64{"a": 400, "b": 600, "c": 0}
	var sum int64
	for _, s := range shares {
		if s.AmountCents != want[s.Name] {
			t.Fatalf("%s = %d, want %d", s.Name, s.AmountCents, want[s.Name])
		}
		sum += s.AmountCents
	}
	if sum != 1000 {
		t.Fatalf("sum = %d, want 1000", sum)
	}
}

// TestFixedShortfallSplitRemainder 固定额之和小于总额：差额在自由参与者间等额分摊并补差。
// 1000 分，a 固定 101，b/c 自由：差额 899，分给 b、c 得 [450,449]。
func TestFixedShortfallSplitRemainder(t *testing.T) {
	shares, err := ComputeShares(1000, ModeFixed, []ParticipantInput{
		{Name: "a", Fixed: ptr(101)}, {Name: "b"}, {Name: "c"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]int64{"a": 101, "b": 450, "c": 449}
	var sum int64
	for _, s := range shares {
		if s.AmountCents != want[s.Name] {
			t.Fatalf("%s = %d, want %d", s.Name, s.AmountCents, want[s.Name])
		}
		sum += s.AmountCents
	}
	if sum != 1000 {
		t.Fatalf("sum = %d, want 1000", sum)
	}
}

// TestFixedShortfallNoFreeRejected 差额存在但无自由参与者分摊，必须拒绝。
func TestFixedShortfallNoFreeRejected(t *testing.T) {
	_, err := ComputeShares(1000, ModeFixed, []ParticipantInput{
		{Name: "a", Fixed: ptr(400)}, {Name: "b", Fixed: ptr(500)},
	})
	if !errors.Is(err, ErrFixedNoFreeRemain) {
		t.Fatalf("want ErrFixedNoFreeRemain, got %v", err)
	}
}

// TestFixedExceedsTotal 固定额之和大于总额必须拒绝。
func TestFixedExceedsTotal(t *testing.T) {
	_, err := ComputeShares(500, ModeFixed, []ParticipantInput{
		{Name: "a", Fixed: ptr(600)},
	})
	if !errors.Is(err, ErrFixedExceedsTotal) {
		t.Fatalf("want ErrFixedExceedsTotal, got %v", err)
	}
}

// TestFixedNegativeRejected 负固定额必须拒绝。
func TestFixedNegativeRejected(t *testing.T) {
	_, err := ComputeShares(1000, ModeFixed, []ParticipantInput{
		{Name: "a", Fixed: ptr(-1)}, {Name: "b"},
	})
	if !errors.Is(err, ErrFixedNegative) {
		t.Fatalf("want ErrFixedNegative, got %v", err)
	}
}

// TestAmountNotPositive 金额非正必须拒绝。
func TestAmountNotPositive(t *testing.T) {
	for _, amt := range []int64{0, -5} {
		_, err := ComputeShares(amt, ModeEqual, []ParticipantInput{{Name: "a"}})
		if !errors.Is(err, ErrAmountNotPositive) {
			t.Fatalf("amount %d: want ErrAmountNotPositive, got %v", amt, err)
		}
	}
}

// TestEmptyParticipantsRejected 参与者列表为空必须拒绝。
func TestEmptyParticipantsRejected(t *testing.T) {
	_, err := ComputeShares(100, ModeEqual, nil)
	if !errors.Is(err, ErrEmptyParticipants) {
		t.Fatalf("want ErrEmptyParticipants, got %v", err)
	}
}

// TestUnknownModeRejected 未知模式必须拒绝。
func TestUnknownModeRejected(t *testing.T) {
	_, err := ComputeShares(100, Mode("weird"), []ParticipantInput{{Name: "a"}})
	if !errors.Is(err, ErrUnknownMode) {
		t.Fatalf("want ErrUnknownMode, got %v", err)
	}
}

// TestSharesAlwaysSumToTotal 各模式各形态下份额之和均严格等于总额。
func TestSharesAlwaysSumToTotal(t *testing.T) {
	cases := []struct {
		name   string
		amount int64
		mode   Mode
		parts  []ParticipantInput
	}{
		{"equal-7-into-3", 7, ModeEqual, []ParticipantInput{{Name: "a"}, {Name: "b"}, {Name: "c"}}},
		{"ratio-prime", 97, ModeRatio, []ParticipantInput{{Name: "a", Weight: 3}, {Name: "b", Weight: 5}, {Name: "c", Weight: 7}}},
		{"ratio-with-zero", 100, ModeRatio, []ParticipantInput{{Name: "a", Weight: 0}, {Name: "b", Weight: 2}, {Name: "c", Weight: 3}}},
		{"fixed-shortfall", 1000, ModeFixed, []ParticipantInput{{Name: "a", Fixed: ptr(250)}, {Name: "b"}, {Name: "c"}, {Name: "d"}}},
		{"fixed-exact", 600, ModeFixed, []ParticipantInput{{Name: "a", Fixed: ptr(100)}, {Name: "b", Fixed: ptr(500)}, {Name: "c"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shares, err := ComputeShares(tc.amount, tc.mode, tc.parts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var sum int64
			for _, s := range shares {
				if s.AmountCents < 0 {
					t.Fatalf("negative share: %+v", s)
				}
				sum += s.AmountCents
			}
			if sum != tc.amount {
				t.Fatalf("sum = %d, want %d", sum, tc.amount)
			}
		})
	}
}

// TestNetBalance 验证净额计算：a 付 900 等额分给 a/b/c，净额为 a=600, b=-300, c=-300。
func TestNetBalance(t *testing.T) {
	members := []string{"a", "b", "c"}
	shares := []Share{{Name: "a", AmountCents: 300}, {Name: "b", AmountCents: 300}, {Name: "c", AmountCents: 300}}
	bills := []Bill{{Payer: "a", AmountCents: 900, Shares: shares}}
	bal := NetBalance(members, bills)
	want := map[string]int64{"a": 600, "b": -300, "c": -300}
	for name, w := range want {
		if bal[name] != w {
			t.Fatalf("%s net = %d, want %d", name, bal[name], w)
		}
	}
	// 净额之和必须为 0。
	var sum int64
	for _, v := range bal {
		sum += v
	}
	if sum != 0 {
		t.Fatalf("net sum = %d, want 0", sum)
	}
}

// TestSettleGreedyWithTieBreak 验证结算的确定性贪心与并列时按名称升序。
func TestSettleGreedyWithTieBreak(t *testing.T) {
	// a=600（债权人），b=-300、c=-300（债务人并列，按名称 b 先选）。
	bal := map[string]int64{"a": 600, "b": -300, "c": -300}
	transfers := Settle(bal)
	if len(transfers) != 2 {
		t.Fatalf("want 2 transfers, got %d: %+v", len(transfers), transfers)
	}
	// 第一笔：b→a 300；第二笔：c→a 300。
	if transfers[0].From != "b" || transfers[0].To != "a" || transfers[0].AmountCents != 300 {
		t.Fatalf("transfer[0] = %+v, want b->a 300", transfers[0])
	}
	if transfers[1].From != "c" || transfers[1].To != "a" || transfers[1].AmountCents != 300 {
		t.Fatalf("transfer[1] = %+v, want c->a 300", transfers[1])
	}
	// 应用转账后净额应全部清零。From 为债务人、To 为债权人，
	// 故 From 净额增加（向 0 靠拢）、To 净额减少。
	after := map[string]int64{"a": 600, "b": -300, "c": -300}
	for _, tr := range transfers {
		after[tr.From] += tr.AmountCents
		after[tr.To] -= tr.AmountCents
	}
	for name, v := range after {
		if v != 0 {
			t.Fatalf("after settle %s = %d, want 0", name, v)
		}
	}
}

// TestSettleDoesNotMutateInput Settle 不得修改传入的净额 map。
func TestSettleDoesNotMutateInput(t *testing.T) {
	bal := map[string]int64{"a": 100, "b": -100}
	_ = Settle(bal)
	if bal["a"] != 100 || bal["b"] != -100 {
		t.Fatalf("input mutated: %+v", bal)
	}
}

// TestSettleZeroBalancesEmpty 净额全为 0 时返回空转账方案。
func TestSettleZeroBalancesEmpty(t *testing.T) {
	bal := map[string]int64{"a": 0, "b": 0, "c": 0}
	if got := Settle(bal); len(got) != 0 {
		t.Fatalf("want no transfers, got %+v", got)
	}
}

// TestSettleComplexMultiCreditor 多债权人场景下仍保证净额清零且转账金额为正。
func TestSettleComplexMultiCreditor(t *testing.T) {
	// a=400, b=200（债权人）；c=-300, d=-300（债务人）。
	bal := map[string]int64{"a": 400, "b": 200, "c": -300, "d": -300}
	transfers := Settle(bal)
	after := map[string]int64{"a": 400, "b": 200, "c": -300, "d": -300}
	for _, tr := range transfers {
		if tr.AmountCents <= 0 {
			t.Fatalf("non-positive transfer: %+v", tr)
		}
		after[tr.From] += tr.AmountCents
		after[tr.To] -= tr.AmountCents
	}
	for name, v := range after {
		if v != 0 {
			t.Fatalf("after settle %s = %d, want 0", name, v)
		}
	}
}
