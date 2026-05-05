# Contract 说明

兼容性来源以官方 `remnawave/node` 2.7.x contract 为准：

```text
tmp/remnawave-node/libs/contract
```

Go 侧公开类型放在：

```text
internal/contracts
```

## Golden 测试

Golden fixture 预留目录：

```text
testdata/contracts/official-2.7.0
```

不要把官方 TypeScript contract 包整体复制进仓库。后续 M0 应从官方 contract 中提取小型请求/响应 JSON fixture，并比较：

- HTTP method 和 path
- JSON 字段名
- response envelope
- `null`、空数组、空对象行为
- 时间字符串格式

## Stub 策略

计划内外部路由已经全部注册，避免已知 Remnawave 调用得到 404。stub 必须显式返回兼容占位数据，并在代码和测试中保留“未实现”的可见边界。
