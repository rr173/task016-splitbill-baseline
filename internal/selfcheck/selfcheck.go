package selfcheck

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"task016-splitbill/internal/httpapi"
	"task016-splitbill/internal/splitbill"
)

// Run 执行无需外部依赖的自检：通过 httptest 启动真实 HTTP 服务，
// 覆盖建组、记账、净额、结算与各类边界约束。成功返回 0，任一失败返回 1。
func Run() int {
	passed, failed := 0, 0
	check := func(name string, fn func() error) {
		if err := fn(); err != nil {
			failed++
			fmt.Printf("FAIL %-36s %v\n", name, err)
		} else {
			passed++
			fmt.Printf("PASS %s\n", name)
		}
	}

	api := httpapi.New(splitbill.NewStore())
	srv := httptest.NewServer(api.Handler())
	defer srv.Close()

	do := func(method, path, body string) (*http.Response, []byte, error) {
		var r io.Reader
		if body != "" {
			r = bytes.NewReader([]byte(body))
		}
		req, err := http.NewRequest(method, srv.URL+path, r)
		if err != nil {
			return nil, nil, err
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, nil, err
		}
		data, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, data, readErr
	}

	createGroup := func(members []string) (string, error) {
		b, _ := json.Marshal(map[string]any{"members": members})
		resp, body, err := do(http.MethodPost, "/groups", string(b))
		if err != nil {
			return "", err
		}
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("create group status=%d body=%s", resp.StatusCode, body)
		}
		var out struct {
			GroupID string `json:"group_id"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return "", err
		}
		return out.GroupID, nil
	}

	type pstruct struct {
		Name   string `json:"name"`
		Weight int64  `json:"weight,omitempty"`
		Fixed  *int64 `json:"fixed,omitempty"`
	}

	addBill := func(groupID string, body string) (int, []byte, error) {
		resp, b, err := do(http.MethodPost, "/groups/"+groupID+"/bills", body)
		return resp.StatusCode, b, err
	}

	getBalance := func(groupID string) (map[string]int64, int, error) {
		resp, body, err := do(http.MethodGet, "/groups/"+groupID+"/balance", "")
		if err != nil {
			return nil, 0, err
		}
		var out struct {
			Balances []struct {
				Name string `json:"name"`
				Net  int64  `json:"net"`
			} `json:"balances"`
		}
		_ = json.Unmarshal(body, &out)
		m := make(map[string]int64, len(out.Balances))
		for _, b := range out.Balances {
			m[b.Name] = b.Net
		}
		return m, resp.StatusCode, nil
	}

	getSettlement := func(groupID string) ([]map[string]any, int, error) {
		resp, body, err := do(http.MethodGet, "/groups/"+groupID+"/settlement", "")
		if err != nil {
			return nil, 0, err
		}
		var out struct {
			Transfers []map[string]any `json:"transfers"`
		}
		_ = json.Unmarshal(body, &out)
		return out.Transfers, resp.StatusCode, nil
	}

	sharesOf := func(body []byte) []shareJSON {
		var out struct {
			Shares []shareJSON `json:"shares"`
		}
		_ = json.Unmarshal(body, &out)
		return out.Shares
	}

	_ = sharesOf

	check("健康检查", func() error {
		resp, _, err := do(http.MethodGet, "/healthz", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status=%d", resp.StatusCode)
		}
		return nil
	})

	var gid string
	check("建组返回组标识", func() error {
		id, err := createGroup([]string{"a", "b", "c"})
		if err != nil {
			return err
		}
		if id == "" {
			return fmt.Errorf("empty group id")
		}
		gid = id
		return nil
	})

	check("等额分摊补差保证总额精确", func() error {
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 1000, "mode": "equal",
			"participants": []pstruct{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		})
		status, body, err := addBill(gid, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", status, body)
		}
		var out struct {
			Shares []shareJSON `json:"shares"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return err
		}
		want := map[string]int64{"a": 334, "b": 333, "c": 333}
		var sum int64
		for _, s := range out.Shares {
			if s.Amount != want[s.Name] {
				return fmt.Errorf("%s=%d want %d", s.Name, s.Amount, want[s.Name])
			}
			sum += s.Amount
		}
		if sum != 1000 {
			return fmt.Errorf("sum=%d want 1000", sum)
		}
		return nil
	})

	check("按比例分摊补差且零权重不参与", func() error {
		id, err := createGroup([]string{"a", "b", "c", "d"})
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 101, "mode": "ratio",
			"participants": []pstruct{
				{Name: "a", Weight: 0}, {Name: "b", Weight: 1}, {Name: "c", Weight: 0}, {Name: "d", Weight: 1},
			},
		})
		status, body, err := addBill(id, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", status, body)
		}
		var out struct {
			Shares []shareJSON `json:"shares"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return err
		}
		want := map[string]int64{"a": 0, "b": 51, "c": 0, "d": 50}
		var sum int64
		for _, s := range out.Shares {
			if s.Amount != want[s.Name] {
				return fmt.Errorf("%s=%d want %d", s.Name, s.Amount, want[s.Name])
			}
			sum += s.Amount
		}
		if sum != 101 {
			return fmt.Errorf("sum=%d want 101", sum)
		}
		return nil
	})

	check("固定额恰好等于总额时自由参与者份额为 0", func() error {
		id, err := createGroup([]string{"a", "b", "c"})
		if err != nil {
			return err
		}
		fixed := int64(400)
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 1000, "mode": "fixed",
			"participants": []pstruct{{Name: "a", Fixed: &fixed}, {Name: "b", Fixed: ptr(int64(600))}, {Name: "c"}},
		})
		status, body, err := addBill(id, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", status, body)
		}
		var out struct {
			Shares []shareJSON `json:"shares"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return err
		}
		want := map[string]int64{"a": 400, "b": 600, "c": 0}
		for _, s := range out.Shares {
			if s.Amount != want[s.Name] {
				return fmt.Errorf("%s=%d want %d", s.Name, s.Amount, want[s.Name])
			}
		}
		return nil
	})

	check("固定额小于总额时差额等额补差", func() error {
		id, err := createGroup([]string{"a", "b", "c"})
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 1000, "mode": "fixed",
			"participants": []pstruct{{Name: "a", Fixed: ptr(int64(101))}, {Name: "b"}, {Name: "c"}},
		})
		status, body, err := addBill(id, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", status, body)
		}
		var out struct {
			Shares []shareJSON `json:"shares"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			return err
		}
		want := map[string]int64{"a": 101, "b": 450, "c": 449}
		var sum int64
		for _, s := range out.Shares {
			if s.Amount != want[s.Name] {
				return fmt.Errorf("%s=%d want %d", s.Name, s.Amount, want[s.Name])
			}
			sum += s.Amount
		}
		if sum != 1000 {
			return fmt.Errorf("sum=%d want 1000", sum)
		}
		return nil
	})

	check("固定额之和大于总额被拒绝", func() error {
		id, err := createGroup([]string{"a"})
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 500, "mode": "fixed",
			"participants": []pstruct{{Name: "a", Fixed: ptr(int64(600))}},
		})
		status, body, err := addBill(id, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400 body=%s", status, body)
		}
		return nil
	})

	check("固定额差额存在但无自由参与者被拒绝", func() error {
		id, err := createGroup([]string{"a", "b"})
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 1000, "mode": "fixed",
			"participants": []pstruct{{Name: "a", Fixed: ptr(int64(400))}, {Name: "b", Fixed: ptr(int64(500))}},
		})
		status, body, err := addBill(id, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400 body=%s", status, body)
		}
		return nil
	})

	check("净额计算正确", func() error {
		// 新组：a 付 900 等额分给 a/b/c，净额 a=600, b=-300, c=-300。
		id, err := createGroup([]string{"a", "b", "c"})
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 900, "mode": "equal",
			"participants": []pstruct{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		})
		status, body, err := addBill(id, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("status=%d body=%s", status, body)
		}
		bal, bstatus, err := getBalance(id)
		if err != nil {
			return err
		}
		if bstatus != http.StatusOK {
			return fmt.Errorf("balance status=%d", bstatus)
		}
		want := map[string]int64{"a": 600, "b": -300, "c": -300}
		for name, w := range want {
			if bal[name] != w {
				return fmt.Errorf("%s net=%d want %d", name, bal[name], w)
			}
		}
		return nil
	})

	check("结算净额清零且并列按名称升序", func() error {
		// a 付 900 等额分给 a/b/c：a=600，b/c=-300 并列，b 先转给 a。
		id, err := createGroup([]string{"a", "b", "c"})
		if err != nil {
			return err
		}
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 900, "mode": "equal",
			"participants": []pstruct{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		})
		if _, _, err := addBill(id, string(b)); err != nil {
			return err
		}
		transfers, status, err := getSettlement(id)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("settlement status=%d", status)
		}
		if len(transfers) != 2 {
			return fmt.Errorf("want 2 transfers, got %d: %+v", len(transfers), transfers)
		}
		if transfers[0]["from"] != "b" || transfers[0]["to"] != "a" || int64(transfers[0]["amount"].(float64)) != 300 {
			return fmt.Errorf("transfer[0]=%+v want b->a 300", transfers[0])
		}
		if transfers[1]["from"] != "c" || transfers[1]["to"] != "a" || int64(transfers[1]["amount"].(float64)) != 300 {
			return fmt.Errorf("transfer[1]=%+v want c->a 300", transfers[1])
		}
		// 应用转账后净额应全部清零。From 为债务人、To 为债权人，
		// 故 From 净额增加（向 0 靠拢）、To 净额减少。
		bal, _, _ := getBalance(id)
		for _, tr := range transfers {
			f := tr["from"].(string)
			to := tr["to"].(string)
			amt := int64(tr["amount"].(float64))
			bal[f] += amt
			bal[to] -= amt
		}
		for name, v := range bal {
			if v != 0 {
				return fmt.Errorf("after settle %s=%d want 0", name, v)
			}
		}
		return nil
	})

	check("多笔账单结算后净额清零", func() error {
		id, err := createGroup([]string{"a", "b", "c", "d"})
		if err != nil {
			return err
		}
		// a 付 400 等额分给 4 人；c 付 200 等额分给 4 人。
		b1, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 400, "mode": "equal",
			"participants": []pstruct{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}},
		})
		b2, _ := json.Marshal(map[string]any{
			"payer": "c", "amount": 200, "mode": "equal",
			"participants": []pstruct{{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"}},
		})
		if _, _, err := addBill(id, string(b1)); err != nil {
			return err
		}
		if _, _, err := addBill(id, string(b2)); err != nil {
			return err
		}
		transfers, status, err := getSettlement(id)
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("settlement status=%d", status)
		}
		bal, _, _ := getBalance(id)
		for _, tr := range transfers {
			f := tr["from"].(string)
			to := tr["to"].(string)
			amt := int64(tr["amount"].(float64))
			if amt <= 0 {
				return fmt.Errorf("non-positive transfer: %+v", tr)
			}
			bal[f] += amt
			bal[to] -= amt
		}
		for name, v := range bal {
			if v != 0 {
				return fmt.Errorf("after settle %s=%d want 0", name, v)
			}
		}
		return nil
	})

	check("金额非正被拒绝", func() error {
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 0, "mode": "equal",
			"participants": []pstruct{{Name: "a"}},
		})
		status, body, err := addBill(gid, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400 body=%s", status, body)
		}
		return nil
	})

	check("付款人非组成员被拒绝", func() error {
		b, _ := json.Marshal(map[string]any{
			"payer": "z", "amount": 100, "mode": "equal",
			"participants": []pstruct{{Name: "a"}},
		})
		status, body, err := addBill(gid, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400 body=%s", status, body)
		}
		return nil
	})

	check("参与者非组成员被拒绝", func() error {
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 100, "mode": "equal",
			"participants": []pstruct{{Name: "a"}, {Name: "z"}},
		})
		status, body, err := addBill(gid, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400 body=%s", status, body)
		}
		return nil
	})

	check("未知组返回 404", func() error {
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 100, "mode": "equal",
			"participants": []pstruct{{Name: "a"}},
		})
		status, _, err := addBill("no-such-group", string(b))
		if err != nil {
			return err
		}
		if status != http.StatusNotFound {
			return fmt.Errorf("status=%d want 404", status)
		}
		resp, _, err := do(http.MethodGet, "/groups/no-such-group/balance", "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("balance status=%d want 404", resp.StatusCode)
		}
		return nil
	})

	check("非法 JSON 被拒绝", func() error {
		status, _, err := addBill(gid, "{not json")
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("多段 JSON 被拒绝", func() error {
		status, _, err := addBill(gid, `{"payer":"a","amount":100,"mode":"equal","participants":[{"name":"a"}]}}`)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("未知字段被拒绝", func() error {
		body := `{"payer":"a","amount":100,"mode":"equal","participants":[{"name":"a"}],"extra":1}`
		status, _, err := addBill(gid, body)
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", status)
		}
		return nil
	})

	check("组内成员重复被拒绝", func() error {
		b, _ := json.Marshal(map[string]any{"members": []string{"a", "a"}})
		resp, _, err := do(http.MethodPost, "/groups", string(b))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400", resp.StatusCode)
		}
		return nil
	})

	check("比例权重全为零被拒绝", func() error {
		b, _ := json.Marshal(map[string]any{
			"payer": "a", "amount": 100, "mode": "ratio",
			"participants": []pstruct{{Name: "a", Weight: 0}, {Name: "b", Weight: 0}},
		})
		status, body, err := addBill(gid, string(b))
		if err != nil {
			return err
		}
		if status != http.StatusBadRequest {
			return fmt.Errorf("status=%d want 400 body=%s", status, body)
		}
		if !strings.Contains(string(body), "权重") {
			return fmt.Errorf("error should mention weight: %s", body)
		}
		return nil
	})

	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

// shareJSON 用于解析记账响应中的份额。
type shareJSON struct {
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
}

// ptr 返回指向 i 的指针，便于构造 fixed 字段。
func ptr(i int64) *int64 { return &i }
