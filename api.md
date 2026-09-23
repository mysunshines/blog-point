# point-service 对外 API 文档

> 自动生成自 `point.proto`（模式：proto）。
> 网关按 `/api/v1/point/<snake_method>` 反射代理到 gRPC 方法 `point.v1.PointService/<Method>`。
> 生成时间：2026-09-23 17:30:38
> Base URL（网关入口）：http://localhost:8081

## 接口列表

| Method | Path | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/v1/point/check_in` | 登录 |  |
| `GET` | `/api/v1/point/get_my_points` | 公开 |  |
| `GET` | `/api/v1/point/get_point_logs` | 公开 |  |
| `GET` | `/api/v1/point/get_checkin_status` | 公开 |  |
| `POST` | `/api/v1/point/admin_list_rules` | 管理员 |  |
| `POST` | `/api/v1/point/admin_create_rule` | 公开 |  |
| `PUT` | `/api/v1/point/admin_update_rule` | 公开 |  |
| `POST` | `/api/v1/point/admin_delete_rule` | 公开 |  |
| `POST` | `/api/v1/point/admin_adjust_points` | 公开 |  |

## CheckIn

- **URL**: `http://localhost:8081/api/v1/point/check_in?code=0&message=<message>&checked_in_today=false&streak=0&last_checkin_date=<last_checkin_date>&code=0&message=<message>&code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0`
- **Method**: `GET`
- **鉴权**: 登录（需 JWT）

### Headers
```http
Authorization: Bearer <token>
Content-Type: application/json
```

### Request
**参数位置**：Query String

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `result` | `CheckInResult` |  | 见 [CheckInResult](#checkinresult) |
| `checked_in_today` | `bool` | 今日是否已签到 | `false` |
| `streak` | `uint32` | 当前连续签到天数（今天未签到但昨天签到了仍保留；断了则为 0） | `0` |
| `last_checkin_date` | `string` | 最近一次签到日期 YYYY-MM-DD（无记录则空） | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `status` | `CheckinStatus` |  | 见 [CheckinStatus](#checkinstatus) |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `point` | `UserPoint` |  | 见 [UserPoint](#userpoint) |
| `page` | `uint32` |  | `0` |
| `page_size` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `logs` | `PointLog[]` |  | [] |
| `total` | `uint32` |  | `0` |
| `event_type` | `string` | checkin / publish_article / weekly_rank / article_purchased ... | `""` |
| `user_id` | `uint32` | 目标用户；为 0 时取当前登录用户 | `0` |
| `context` | `string` | JSON 上下文，供 condition 匹配（如 {"streak":7,"rank":3}） | `""` |
| `related_type` | `string` | article / background ... | `""` |
| `related_id` | `uint32` |  | `0` |
| `idempotency_key` | `string` | 可选：幂等键；为空时由服务端按 user+event+related 派生 | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `points` | `int64` | 实际发放积分 | `0` |
| `rules` | `string[]` | 命中的规则 code 列表 | `[]` |
| `user_id` | `uint32` | 为 0 时取当前登录用户 | `0` |
| `amount` | `int64` | 消费积分（正数） | `0` |
| `item_type` | `string` | article / background | `""` |
| `item_id` | `uint32` |  | `0` |
| `remark` | `string` |  | `""` |
| `idempotency_key` | `string` | 可选：幂等键；为空时由服务端按 user+item 派生 | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 消费后余额 | `0` |
| `user_id` | `uint32` | 为 0 时取当前登录用户 | `0` |
| `item_type` | `string` | article / background | `""` |
| `item_id` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `purchased` | `bool` |  | `false` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rules` | `PointRule[]` |  | [] |
| `code` | `string` |  | `""` |
| `name` | `string` |  | `""` |
| `event_type` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `name` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `status` | `uint32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `user_id` | `uint32` |  | `0` |
| `amount` | `int64` | 正=发放，负=扣减 | `0` |
| `remark` | `string` |  | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 调整后余额 | `0` |

**Query 示例**：
```json
code=0&message=<message>&checked_in_today=false&streak=0&last_checkin_date=<last_checkin_date>&code=0&message=<message>&code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `result` | `CheckInResult` |  | 见 [CheckInResult](#checkinresult) |

**Response 示例**：
```json
{"code": 0, "message": "success", "result": {}}
```

### curl 示例
```bash
curl -X GET 'http://localhost:8081/api/v1/point/check_in?code=0&message=<message>&checked_in_today=false&streak=0&last_checkin_date=<last_checkin_date>&code=0&message=<message>&code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0' \
  -H 'Authorization: Bearer <token>'
```

## GetMyPoints

- **URL**: `http://localhost:8081/api/v1/point/get_my_points?code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0`
- **Method**: `GET`
- **鉴权**: 公开（无需鉴权）

### Headers
```http
Content-Type: application/json
```

### Request
**参数位置**：Query String

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `point` | `UserPoint` |  | 见 [UserPoint](#userpoint) |
| `page` | `uint32` |  | `0` |
| `page_size` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `logs` | `PointLog[]` |  | [] |
| `total` | `uint32` |  | `0` |
| `event_type` | `string` | checkin / publish_article / weekly_rank / article_purchased ... | `""` |
| `user_id` | `uint32` | 目标用户；为 0 时取当前登录用户 | `0` |
| `context` | `string` | JSON 上下文，供 condition 匹配（如 {"streak":7,"rank":3}） | `""` |
| `related_type` | `string` | article / background ... | `""` |
| `related_id` | `uint32` |  | `0` |
| `idempotency_key` | `string` | 可选：幂等键；为空时由服务端按 user+event+related 派生 | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `points` | `int64` | 实际发放积分 | `0` |
| `rules` | `string[]` | 命中的规则 code 列表 | `[]` |
| `user_id` | `uint32` | 为 0 时取当前登录用户 | `0` |
| `amount` | `int64` | 消费积分（正数） | `0` |
| `item_type` | `string` | article / background | `""` |
| `item_id` | `uint32` |  | `0` |
| `remark` | `string` |  | `""` |
| `idempotency_key` | `string` | 可选：幂等键；为空时由服务端按 user+item 派生 | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 消费后余额 | `0` |
| `user_id` | `uint32` | 为 0 时取当前登录用户 | `0` |
| `item_type` | `string` | article / background | `""` |
| `item_id` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `purchased` | `bool` |  | `false` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rules` | `PointRule[]` |  | [] |
| `code` | `string` |  | `""` |
| `name` | `string` |  | `""` |
| `event_type` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `name` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `status` | `uint32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `user_id` | `uint32` |  | `0` |
| `amount` | `int64` | 正=发放，负=扣减 | `0` |
| `remark` | `string` |  | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 调整后余额 | `0` |

**Query 示例**：
```json
code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `point` | `UserPoint` |  | 见 [UserPoint](#userpoint) |

**Response 示例**：
```json
{"code": 0, "message": "success", "point": 0}
```

### curl 示例
```bash
curl -X GET 'http://localhost:8081/api/v1/point/get_my_points?code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0'
```

## GetPointLogs

- **URL**: `http://localhost:8081/api/v1/point/get_point_logs?page=0&page_size=0`
- **Method**: `GET`
- **鉴权**: 公开（无需鉴权）

### Headers
```http
Content-Type: application/json
```

### Request
**参数位置**：Query String

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `page` | `uint32` |  | `0` |
| `page_size` | `uint32` |  | `0` |

**Query 示例**：
```json
page=0&page_size=0
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `logs` | `PointLog[]` |  | [] |
| `total` | `uint32` |  | `0` |

**Response 示例**：
```json
{"code": 0, "message": "success", "logs": [], "total": 0}
```

### curl 示例
```bash
curl -X GET 'http://localhost:8081/api/v1/point/get_point_logs?page=0&page_size=0'
```

## GetCheckinStatus

- **URL**: `http://localhost:8081/api/v1/point/get_checkin_status?code=0&message=<message>&code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0`
- **Method**: `GET`
- **鉴权**: 公开（无需鉴权）

### Headers
```http
Content-Type: application/json
```

### Request
**参数位置**：Query String

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `status` | `CheckinStatus` |  | 见 [CheckinStatus](#checkinstatus) |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `point` | `UserPoint` |  | 见 [UserPoint](#userpoint) |
| `page` | `uint32` |  | `0` |
| `page_size` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `logs` | `PointLog[]` |  | [] |
| `total` | `uint32` |  | `0` |
| `event_type` | `string` | checkin / publish_article / weekly_rank / article_purchased ... | `""` |
| `user_id` | `uint32` | 目标用户；为 0 时取当前登录用户 | `0` |
| `context` | `string` | JSON 上下文，供 condition 匹配（如 {"streak":7,"rank":3}） | `""` |
| `related_type` | `string` | article / background ... | `""` |
| `related_id` | `uint32` |  | `0` |
| `idempotency_key` | `string` | 可选：幂等键；为空时由服务端按 user+event+related 派生 | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `points` | `int64` | 实际发放积分 | `0` |
| `rules` | `string[]` | 命中的规则 code 列表 | `[]` |
| `user_id` | `uint32` | 为 0 时取当前登录用户 | `0` |
| `amount` | `int64` | 消费积分（正数） | `0` |
| `item_type` | `string` | article / background | `""` |
| `item_id` | `uint32` |  | `0` |
| `remark` | `string` |  | `""` |
| `idempotency_key` | `string` | 可选：幂等键；为空时由服务端按 user+item 派生 | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 消费后余额 | `0` |
| `user_id` | `uint32` | 为 0 时取当前登录用户 | `0` |
| `item_type` | `string` | article / background | `""` |
| `item_id` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `purchased` | `bool` |  | `false` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rules` | `PointRule[]` |  | [] |
| `code` | `string` |  | `""` |
| `name` | `string` |  | `""` |
| `event_type` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `name` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `status` | `uint32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `user_id` | `uint32` |  | `0` |
| `amount` | `int64` | 正=发放，负=扣减 | `0` |
| `remark` | `string` |  | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 调整后余额 | `0` |

**Query 示例**：
```json
code=0&message=<message>&code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `status` | `CheckinStatus` |  | 见 [CheckinStatus](#checkinstatus) |

**Response 示例**：
```json
{"code": 0, "message": "success", "status": {}}
```

### curl 示例
```bash
curl -X GET 'http://localhost:8081/api/v1/point/get_checkin_status?code=0&message=<message>&code=0&message=<message>&page=0&page_size=0&code=0&message=<message>&total=0&event_type=<event_type>&user_id=0&context=<context>&related_type=<related_type>&related_id=0&idempotency_key=<idempotency_key>&code=0&message=<message>&points=0&rules=<rules>&user_id=0&amount=0&item_type=<item_type>&item_id=0&remark=<remark>&idempotency_key=<idempotency_key>&code=0&message=<message>&balance=0&user_id=0&item_type=<item_type>&item_id=0&code=0&message=<message>&purchased=false&code=0&message=<message>&code=<code>&name=<name>&event_type=<event_type>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&sort=0&code=0&message=<message>&rule_id=0&name=<name>&condition=<condition>&points=0&limit_type=<limit_type>&limit_count=0&status=0&sort=0&code=0&message=<message>&rule_id=0&code=0&message=<message>&user_id=0&amount=0&remark=<remark>&code=0&message=<message>&balance=0'
```

## AdminListRules

- **URL**: `http://localhost:8081/api/v1/point/admin_list_rules`
- **Method**: `POST`
- **鉴权**: 管理员（需 JWT + 管理员角色）

### Headers
```http
Authorization: Bearer <token>
Content-Type: application/json
```

### Request
**参数位置**：Request Body（JSON）

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rules` | `PointRule[]` |  | [] |
| `code` | `string` |  | `""` |
| `name` | `string` |  | `""` |
| `event_type` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `name` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `status` | `uint32` |  | `0` |
| `sort` | `int32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |
| `rule_id` | `uint32` |  | `0` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `user_id` | `uint32` |  | `0` |
| `amount` | `int64` | 正=发放，负=扣减 | `0` |
| `remark` | `string` |  | `""` |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 调整后余额 | `0` |

**Body 示例**：
```json
{"code": 0, "message": "", "rules": [], "code": "", "name": "", "event_type": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "sort": 0, "code": 0, "message": "", "rule": 0, "rule_id": 0, "name": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "status": 0, "sort": 0, "code": 0, "message": "", "rule": 0, "rule_id": 0, "code": 0, "message": "", "user_id": 0, "amount": 0, "remark": "", "code": 0, "message": "", "balance": 0}
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rules` | `PointRule[]` |  | [] |

**Response 示例**：
```json
{"code": 0, "message": "success", "rules": []}
```

### curl 示例
```bash
curl -X POST 'http://localhost:8081/api/v1/point/admin_list_rules' \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{"code": 0, "message": "", "rules": [], "code": "", "name": "", "event_type": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "sort": 0, "code": 0, "message": "", "rule": 0, "rule_id": 0, "name": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "status": 0, "sort": 0, "code": 0, "message": "", "rule": 0, "rule_id": 0, "code": 0, "message": "", "user_id": 0, "amount": 0, "remark": "", "code": 0, "message": "", "balance": 0}'
```

## AdminCreateRule

- **URL**: `http://localhost:8081/api/v1/point/admin_create_rule`
- **Method**: `POST`
- **鉴权**: 公开（无需鉴权）

### Headers
```http
Content-Type: application/json
```

### Request
**参数位置**：Request Body（JSON）

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `string` |  | `""` |
| `name` | `string` |  | `""` |
| `event_type` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `sort` | `int32` |  | `0` |

**Body 示例**：
```json
{"code": "", "name": "", "event_type": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "sort": 0}
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |

**Response 示例**：
```json
{"code": 0, "message": "success", "rule": 0}
```

### curl 示例
```bash
curl -X POST 'http://localhost:8081/api/v1/point/admin_create_rule' \
  -H 'Content-Type: application/json' \
  -d '{"code": "", "name": "", "event_type": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "sort": 0}'
```

## AdminUpdateRule

- **URL**: `http://localhost:8081/api/v1/point/admin_update_rule`
- **Method**: `PUT`
- **鉴权**: 公开（无需鉴权）

### Headers
```http
Content-Type: application/json
```

### Request
**参数位置**：Request Body（JSON）

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `rule_id` | `uint32` |  | `0` |
| `name` | `string` |  | `""` |
| `condition` | `string` |  | `""` |
| `points` | `int64` |  | `0` |
| `limit_type` | `string` |  | `""` |
| `limit_count` | `int32` |  | `0` |
| `status` | `uint32` |  | `0` |
| `sort` | `int32` |  | `0` |

**Body 示例**：
```json
{"rule_id": 0, "name": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "status": 0, "sort": 0}
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `rule` | `PointRule` |  | 见 [PointRule](#pointrule) |

**Response 示例**：
```json
{"code": 0, "message": "success", "rule": 0}
```

### curl 示例
```bash
curl -X PUT 'http://localhost:8081/api/v1/point/admin_update_rule' \
  -H 'Content-Type: application/json' \
  -d '{"rule_id": 0, "name": "", "condition": "", "points": 0, "limit_type": "", "limit_count": 0, "status": 0, "sort": 0}'
```

## AdminDeleteRule

- **URL**: `http://localhost:8081/api/v1/point/admin_delete_rule`
- **Method**: `POST`
- **鉴权**: 公开（无需鉴权）

### Headers
```http
Content-Type: application/json
```

### Request
**参数位置**：Request Body（JSON）

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `rule_id` | `uint32` |  | `0` |

**Body 示例**：
```json
{"rule_id": 0}
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |

**Response 示例**：
```json
{"code": 0, "message": "success"}
```

### curl 示例
```bash
curl -X POST 'http://localhost:8081/api/v1/point/admin_delete_rule' \
  -H 'Content-Type: application/json' \
  -d '{"rule_id": 0}'
```

## AdminAdjustPoints

- **URL**: `http://localhost:8081/api/v1/point/admin_adjust_points`
- **Method**: `POST`
- **鉴权**: 公开（无需鉴权）

### Headers
```http
Content-Type: application/json
```

### Request
**参数位置**：Request Body（JSON）

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `user_id` | `uint32` |  | `0` |
| `amount` | `int64` | 正=发放，负=扣减 | `0` |
| `remark` | `string` |  | `""` |

**Body 示例**：
```json
{"user_id": 0, "amount": 0, "remark": ""}
```

### Response
| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `code` | `uint32` |  | `0` |
| `message` | `string` |  | `""` |
| `balance` | `int64` | 调整后余额 | `0` |

**Response 示例**：
```json
{"code": 0, "message": "success", "balance": 0}
```

### curl 示例
```bash
curl -X POST 'http://localhost:8081/api/v1/point/admin_adjust_points' \
  -H 'Content-Type: application/json' \
  -d '{"user_id": 0, "amount": 0, "remark": ""}'
```

---

## 数据结构

> 下列 message / enum 被上述接口的请求或响应引用；结构体字段中的 message 类型可点击跳转到对应定义。

### CheckInResult

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `streak` | `uint32` | 本次签到后的连续天数 | `0` |
| `points` | `int64` | 本次获得积分 | `0` |
| `rules` | `string[]` | 命中的规则 code 列表 | `[]` |

### CheckinStatus

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `checked_in_today` | `bool` | 今日是否已签到 | `false` |
| `streak` | `uint32` | 当前连续签到天数（今天未签到但昨天签到了仍保留；断了则为 0） | `0` |
| `last_checkin_date` | `string` | 最近一次签到日期 YYYY-MM-DD（无记录则空） | `""` |

### UserPoint

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `user_id` | `uint32` |  | `0` |
| `balance` | `int64` | 当前可用积分 | `0` |
| `total_earned` | `int64` | 累计获得 | `0` |
| `total_spent` | `int64` | 累计消费 | `0` |
| `updated_at` | `string` |  | `""` |

### PointLog

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `id` | `uint32` |  | `0` |
| `user_id` | `uint32` |  | `0` |
| `amount` | `int64` | 正=获得，负=消费 | `0` |
| `balance_after` | `int64` | 变动后余额 | `0` |
| `type` | `string` | earn / spend / admin_adjust | `""` |
| `rule_code` | `string` | 命中的规则 code（空表示非规则触发） | `""` |
| `related_type` | `string` | article / background ... | `""` |
| `related_id` | `uint32` |  | `0` |
| `remark` | `string` |  | `""` |
| `created_at` | `string` |  | `""` |

### PointRule

| 字段 | 类型 | 说明 | 示例 |
| --- | --- | --- | --- |
| `id` | `uint32` |  | `0` |
| `code` | `string` | 唯一标识，如 checkin_daily | `""` |
| `name` | `string` | 展示名 | `""` |
| `event_type` | `string` | 触发事件：checkin / publish_article / weekly_rank ... | `""` |
| `condition` | `string` | JSON 条件，如 {"streak":7}、{"rank_lte":10} | `""` |
| `points` | `int64` | 奖励积分 | `0` |
| `limit_type` | `string` | none / daily / weekly / once（防刷） | `""` |
| `limit_count` | `int32` | 限次窗口内最多发放次数 | `0` |
| `status` | `uint32` | 1=启用 0=停用 | `0` |
| `sort` | `int32` |  | `0` |
| `created_at` | `string` |  | `""` |
| `updated_at` | `string` |  | `""` |

