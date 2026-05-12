# 官方 2.7.0 Contract Golden 文件

本目录用于存放从以下位置提取的小型 JSON golden fixture：

```text
tmp/remnawave-node/libs/contract
```

`panel-api.json` 保存 Go contract 测试需要的少量请求和响应样例。`upstream-contract.sha256.json` 保存官方 `remnawave/node 2.7.0` 的 Panel-facing contract 文件路径和 SHA-256，用于后续检查上游 contract 是否漂移。

hash baseline 覆盖：

- `libs/contract/api`
- `libs/contract/commands`
- `libs/contract/constants/errors`
- `libs/contract/constants/xray`
- `libs/contract/models`

`index.ts`、`package.json`、`tsconfig.json` 和非 `.ts` 文件不纳入 baseline。

本地检查当前官方 tag：

```sh
mise run contract-diff
```

检查新 tag：

```sh
CONTRACT_TAG=2.7.1 mise run contract-diff
```

重新生成 baseline 时，先人工确认 Go contract、route 和 golden fixture 已按官方变化更新，再运行：

```sh
mise exec -- go run ./cmd/contract-diff -tag 2.7.0 -source-dir tmp/remnawave-node -baseline testdata/contracts/official-2.7.0/upstream-contract.sha256.json -write-baseline
```

不要把官方 TypeScript contract 包整体复制进仓库。这里只保存必要的小型 JSON fixture 和 hash baseline。
