# magpie-empty-enum-fix（中文说明）

针对 [yetone/magpie](https://github.com/yetone/magpie) 的一个小 bug 的独立仓库：
`plainSchema()` 把工具 schema 里 `enum` 的空字符串原样传给 Antigravity，Google
以 `enum[0]: cannot be empty` 拒绝整个请求。

完整英文说明见 [README.md](README.md)。

## 结论先说

这不是账号风控、不是配额、不是地区限制，也不是请求体太大。触发条件只有一个：
工具 schema 里存在 `enum: ["", …]` 或 `const: ""`。

- `internal/gateway/codeassist.go` 的 `plainSchema()` 只把**非字符串**成员排除，
  `""` 是合法 Go 字符串，于是被保留并送出。
- 同一个函数还把字符串 `const` 转成单元素 `enum`，所以 `const: ""` 会产生一模一样的
  非法载荷；只修 `enum` 分支不够。
- magpie 把这条 400 归为 `shapeRefused` 后换成员重试，最终仍返回 200，但首字延迟
  明显变长、Antigravity 席位被浪费。所以从面板上看像「报错率高」，本质是 schema 校验
  失败 + 慢速降级。
- 2026-11-04 单机台账：带该工具的 29 条记录里 29 条至少吃过一次 Antigravity 400；
  成功的 15 条 Antigravity 记录里 0 条带该工具；另一不带此工具的调用方 75 次里 74 次
  直接成功。这是单机单日证据，不是通用性能结论。

## 修法

两处 `if s, ok := ….(string); ok` 各加 `&& s != ""`。**丢掉整个 `enum` 关键字**
（沿用 `plainSchema` 对「含非字符串成员」的既有规则），而不是把空串从数组里滤掉——
后者模型仍可能回答 `""`，被二次拒绝。

## 目录

| 路径 | 内容 |
| --- | --- |
| `patches/empty-enum.patch` | 纯 diff，`git apply` / `patch -p1` 用 |
| `patches/0001-…patch` | `git format-patch` mbox，带完整提交信息，`git am` 用 |
| `tests/plain_schema_empty_enum_test.go` | 补丁新增的回归测试副本，须在 magpie 的 `internal/gateway` 内运行 |
| `selfcheck/` | 独立、零依赖 Go 模块，在本仓库就能跑 |
| `scripts/verify.sh` | 本仓库一键验收 |
| `scripts/apply.sh` | 对你指定的 magpie 副本做 `--check` / `--apply` / `--am` |
| `.github/workflows/ci.yml` | CI：gofmt、vet、selfcheck、补丁完整性 |
| `docs/VERIFICATION.md` | 逐条命令与真实输出 |
| `docs/UPSTREAM-REPORT.md` | 可直接贴到 magpie Discord / issue 的英文报告 |
| `LICENSE` / `NOTICE` | 本仓库自有内容 MIT；magpie 归属说明 |

## 跑测试

```bash
./scripts/verify.sh
# 或
cd selfcheck && go test ./... -count=1 && go vet ./...
```

`selfcheck/` 不含 magpie 源码，只是对那两个分支的隔离建模：不需要网络、不需要网关、
不消耗 token。它同时校验两份补丁都带了两处 guard、以及上游回归测试文件确实在仓库里。

真正对着原函数的测试是 `tests/plain_schema_empty_enum_test.go`，必须在 magpie
checkout 里跑。

## 应用补丁

必须显式给出 magpie 副本路径；脚本不会默认指向正在运行的实例，不碰
`~/.config/magpie`，不重启服务。

```bash
./scripts/apply.sh /path/to/magpie --check   # 只读检查
./scripts/apply.sh /path/to/magpie --apply   # 应用到工作区，不提交
./scripts/apply.sh /path/to/magpie --am      # 带完整提交信息落地为一次提交
```

基线 commit：`a3768cdd1113118998863ed36bd6dd77c8e9e2cc`。上游若已改动
`internal/gateway/codeassist.go`，请手工 rebase 两个 hunk，不要强推 3-way 合并。

## 验证情况

`docs/VERIFICATION.md` 记录了 Go 1.27.0 / windows-amd64、隔离稀疏克隆上的真实输出：
无补丁时 `TestPlainSchemaDropsEnumWithEmptyValue` 因 `enum:[""]` 失败、有补丁时通过；
`gofmt` / `go vet ./internal/gateway/` / `go build ./internal/gateway/` 通过；
`git apply --check` / `git apply` / `patch --dry-run -p1` 在干净副本上通过；整包 9 个
既有红灯在「换回原版」后逐条一致，与本次改动无关；全仓与跨平台构建**未跑**。
全程未连活动网关、未碰生产配置、未发任何模型请求。

## 上游与许可

magpie 的 `README.md` 明确写着「**This repository doesn't take pull requests.**
Only maintainers can open them.」因此这里**没有 PR**，只提供描述 + diff；
`docs/UPSTREAM-REPORT.md` 可直接贴到 Discord / issue。按 magpie 的
`docs/contributors.md`，维护者若采用根因、错误文本或测试夹具，请在 commit 里 credit
报告者。

已公开发布在 https://github.com/Gruu6y/magpie-empty-enum-fix （public / MIT），
`main` = `38ec986`，GitHub Actions 的 `selfcheck` 每次 push 自动跑。自有材料
MIT（LICENSE），补丁改的是 magpie 自己的 MIT 代码、此处只分发 diff，详见 `NOTICE`。
