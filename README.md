# task016-splitbill

费用分摊结算服务，仅使用标准库实现等额/按比例/按固定额三种分摊模式的份额计算、成员净额累计与净额清零结算方案，不依赖任何第三方库、数据库或外部服务。

## 本地运行

```bash
go run . server --addr :8080
go run . --smoke-test
```

主要接口：

- `GET /healthz`：健康检查。
- `POST /groups`：提交 `{"members":["a","b","c"]}`，返回 `{"group_id":"1"}`。
- `POST /groups/{id}/bills`：提交账单（付款人、金额、模式、参与者及其权重或固定额），返回各参与者分摊份额与账单标识。
- `GET /groups/{id}/balance`：返回组内每个成员的累计净额（已付金额减去应承担份额）。
- `GET /groups/{id}/settlement`：返回使净额清零的转账方案（每笔含付款方、收款方、金额）。

分摊模式：
- `equal`：等额分摊。
- `ratio`：按权重比例分摊，权重为 0 的参与者份额为 0 且不参与舍入补差。
- `fixed`：按固定额分摊；固定额之和恰好等于总额时自由参与者份额为 0，小于总额时差额在自由参与者间等额补差，大于总额时拒绝。

金额一律以分为单位的正整数；所有分摊份额之和严格等于账单总额，舍入差额按参与者顺序依次补 1 分。

## Docker

镜像使用国内 DaoCloud Go 1.26.3 Bookworm builder 和 Alpine 3.20 runtime；支持 `linux/amd64` 与 `linux/arm64` 双架构。
