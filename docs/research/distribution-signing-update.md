# 分发、签名与自动更新方案（工单 #12）

调研日期：2026-11-06。前置输入：`research/mygo-capabilities` 分支 `docs/research/mygo-capabilities.md` §1「分发」（不重复盘点，本文只取分发相关事实并展开）。除标注「推断」外均为事实，逐条给来源。

上游 mygo 文档链接（clone 于 /tmp/mygo-src，main 分支）：
- 构建/打包/签名：https://github.com/egoist/mygo/blob/main/docs/distribution.md
- 自动更新：https://github.com/egoist/mygo/blob/main/docs/updates.md
- 官方 CI workflow：https://github.com/egoist/mygo/blob/main/docs/github-actions.md

## 1. mygo 打包产物形态与各平台输出（问题①）

`mygo build`（`go tool mygo build`，native UI 项目）一次产出全部分发物，输出在 `build/<os>-<arch>/`：

| 平台 | 产物 | 一次性安装 | 更新归档/清单 |
|---|---|---|---|
| macOS | `Remit.app`（签名）+ `Remit-x.y.z[-arm64].dmg`（可签名+公证，拖入 Applications 的窗口） | 配置 updates 后另有 `install.sh`（一行安装到 /Applications，`curl \| sh`） | `remit-x.y.z-darwin-[arm64/universal].tar.gz` + `update-darwin-*.json` + bsdiff `.delta` |
| Windows | `remit.exe` + 应用文件 + NSIS 安装器 `remit-setup-x.y.z-[amd64/arm64].exe`（按用户装到 `%LOCALAPPDATA%\Programs`，免管理员，注册快捷方式/深链/文件关联，带卸载器） | `install.ps1`（PowerShell 5.1/7，`irm \| iex`，校验 SHA256SUMS 后静默安装） | 同上 windows 架构对应文件 |
| Linux | `remit` 二进制 + `.desktop` + 图标 + `remit-x.y.z-linux-<arch>.tar.gz` + `install.sh`（免 root 装 `~/.local/`）+ `.deb`（装 `/opt/remit`）+ Arch `PKGBUILD` | `install.sh` 一行安装并自更新 | 同上 linux 架构对应文件 |

另附 `SHA256SUMS`（安装器/归档/脚本全量）。来源：distribution.md「Platforms」「The installer」「The install script」「Install scripts for macOS and Windows」，updates.md「Build and publish」。

关键事实：

- **交叉编译零障碍**：默认 `CGO_ENABLED=0`，任意机器可编全平台（`-platform darwin/universal,windows/amd64,windows/arm64,linux/amd64,linux/arm64`）；仅 mac 签名/公证/DMG 必须 macOS runner，Windows 安装器需 NSIS（Linux CI `apt install nsis` 即可）。来源：distribution.md「Platforms」、github-actions.md。
- **官方发布流水线现成**：docs/github-actions.md 提供完整 release.yml——推 tag → 三平台矩阵并行构建 → 各自 `-upload` 到同一 draft release → 发布。签名 secrets：`MYGO_UPDATER_PRIVATE_KEY`、`MYGO_WINDOWS_CERTIFICATE_PASSWORD`、mac 证书+公证凭据进 runner keychain。
- **发布动作一键**：`mygo build -upload` 把安装器+更新文件传 GitHub draft release（或 S3/R2）。来源：distribution.md「Publishing」。
- Linux 系统包安装（.deb/AUR，装在 `/opt`）由包管理器负责升级，此时 mygo.Updater `Enabled()` 为 false，不越权自更新。来源：updates.md 文末。
- `identifier`（反域名）与 Linux `slug` 一经发布不可改（权限/包名/更新路径都挂在上面）。来源：distribution.md「Name, icon and version」。

## 2. mac 签名+公证、win 签名：流程、年成本、无签名的体验代价（问题②）

### 2.1 macOS：Developer ID + 公证

流程（mygo 原生，三行配置）：

```ts
macos: {
  signingIdentity: "Developer ID Application: <名> (TEAMID)",
  notarize: { keychainProfile: "notary" },
}
```

`mygo build` 用 hardened runtime + 安全时间戳签名（含资源内所有 Mach-O/嵌套 bundle，从内向外），提交 DMG 给 Apple notary service，回票 staple 到 DMG 和 app；凭据一次存入 `xcrun notarytool store-credentials notary`。CI 上有逐行官方脚本（证书 .p12 进临时 keychain）。来源：distribution.md「Signing and notarization」「Helper executables」、github-actions.md「Signing and notarizing macOS apps」。

成本：**Apple Developer Program 99 USD/年**（个人或组织均可，含 Developer ID 证书与 notary 服务，公证无按次费用）。来源：Apple 官方 https://developer.apple.com/programs/whats-included/ （"99 USD per membership year"）；公证要求见 https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution 。

无签名的体验代价（Gatekeeper）：

- 默认 ad-hoc 签名的 app 只能在构建机上跑，**别的 Mac 上被 Gatekeeper 拦**（Apple silicon 根本不跑无签名代码）。来源：distribution.md「Signing and notarization」。
- macOS Catalina（10.15）起默认要求公证；未公证/未识别开发者 → 弹「Apple cannot check … for malicious software」。来源：Apple 支持 https://support.apple.com/en-us/102445 。
- 绕过路径只剩：先点开一次 → 系统设置 → 隐私与安全性 → 拉到底点「仍要打开」→ 再在弹窗里点「打开」，共两跳+二次确认（现行官方指引 2026-05 更新版只给这条路径；早期「右键→打开」捷径已不在官方文档中）。来源：同上 102445。
- 例外：`install.sh` 用 `curl` 下载不写隔离属性（quarantine），装完**直接打开、无 Gatekeeper 提示**——无签名先发时这是 mac 上最顺的安装路径。来源：distribution.md「Install scripts for macOS and Windows」。

### 2.2 Windows：Authenticode 代码签名

2026 年格局（Microsoft Learn 官方对照表，2026-08-29 更新）：

| 选项 | 成本 | SmartScreen 行为 | 备注 |
|---|---|---|---|
| Microsoft Store（MSIX） | 免费（Store 重签名） | 无警告 | Remit 是独立分发，不适用 |
| Azure Artifact Signing（原 Trusted Signing） | ~$9.99/月 | 有信誉积累期，初期仍可能弹警告 | **个人开发者仅限美国/加拿大**；组织限美/加/EU/UK。对个人（非美加）不可用 |
| OV 证书（DigiCert/Sectigo 等） | $150–300/年 | 同上，按发布者身份积累信誉 | 2023-06 起 CA/B Forum 强制私钥放硬件 token/HSM |
| EV 证书 | $400+/年 | **2024 起不再即时豁免 SmartScreen**，与 OV 同待遇 | 微软明确「不再推荐为绕过 SmartScreen 而买 EV」 |
| 无签名 | 免费 | 「Windows protected your PC」强警告，用户须 More info→Run anyway；企业策略可完全拦截；每个新版本哈希从零积累信誉 | Win11 Smart App Control（默认开于全新安装）直接拦无签名且无正向信誉的文件 |
| SignPath Foundation（开源专属） | **免费** | OV 级（有积累期） | 证书发给项目而非个人，验证「二进制确实构建自你的开源仓库」，密钥在其 HSM，CI 集成，需申请 |
| Certum Open Source Code Signing | **€25**（激活码；需自备其加密卡+读卡器，另有 SimplySign 云版） | OV 级（Certum 页面自述可积累 SmartScreen 信誉） | 个人「Open Source Developer」名义 |

来源：
- Microsoft Learn「Code signing options for Windows app developers」：https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options
- Microsoft Learn「SmartScreen reputation for Windows app developers」：https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/smartscreen-reputation （EV 失去即时豁免、无签名每版从零积累、Smart App Control、企业拦截）
- Azure 定价页（Basic 档 5,000 签名/月）：https://azure.microsoft.com/en-us/pricing/details/artifact-signing/
- SignPath Foundation：https://signpath.org/ （"For OSS projects, our services are free of charge"）
- Certum：https://shop.certum.eu/open-source-code-signing-code.html

mygo 接入：`windows.certificate`（.pfx，CI 上 osslsigncode 签，Linux runner 即可）；硬件 token/云签名（如 signtool /sign /tr …）走 `windows.signCommand`，此时建议 Windows runner。签名范围含主 exe、资源内 helper exe/DLL、安装器与卸载器；`timestampUrl` 默认 DigiCert。来源：distribution.md「Code signing」、github-actions.md「Signing Windows apps」「Signing on Windows」。

无签名代价小结（相对签名版）：下载即「已保护你的电脑」全屏蓝框、UAC 显示 Unknown Publisher、企业环境可能无 Run anyway 按钮、Smart App Control 机器直接拒绝、且**每次发新版都从零信誉重来**（签名版可沿发布者证书继承信誉）。来源：smartscreen-reputation 文档。

## 3. 开源自动更新惯用法（问题③）

桌面开源应用的更新方案三类惯用法，与 mygo 内置能力一一对应：

| 惯用法 | 代表 | 机制 | mygo 内置等价物 |
|---|---|---|---|
| Sparkle 类框架 | Sparkle 2（macOS 标杆，9.9k★）：EdDSA 签名 + Apple 代码签名双重校验、RSS appcast 清单、bsdiff 式 delta、beta 频道、HTTPS 托管，要求 macOS 12+ | 客户端拉清单 → 验签 → 后台下载打补丁 → 重启 | `plugins/updater`：官方自称「Mac 用户从 Sparkle 认识的更新窗口，全平台都有」；后台检查、release notes、安装/跳过/稍后 |
| 自建 manifest + 自有服务器 | 自托管 JSON + Ed25519 验签 + S3/R2 分发 | 同上，通道换成自己域名 | `updates.url` + `updates.s3`（S3/Cloudflare R2，清单 no-cache 上传，免额外工具）；`mygo.Updater.SetSource` 支持带鉴权/自选频道的自定义源 |
| GitHub Releases 通道 | `releases/latest/download/<asset>` 官方固定链接是开源分发事实标准 | 客户端读 latest release 的清单 | `updates.github: "owner/repo"`：`mygo build -upload` 建 draft release，app 读各平台 manifest；prerelease 频道用 GitHub API（限 60 次/时/地址，实际一次检查一请求）；`tagPrefix` 可与 CLI 等共用仓库 |

来源：Sparkle https://github.com/sparkle-project/Sparkle （README：EdDSA+Code Signing、delta、channels、HTTPS 要求、macOS 12+）；GitHub 固定链接 https://docs.github.com/en/repositories/releasing-projects-on-github/linking-to-releases ；mygo updates.md 全文（签名更新链路：`mygo keygen` 生成 Ed25519 密钥对 → `MYGO_UPDATER_PRIVATE_KEY` 参与构建 → 产物清单含 URL/size/签名 → app 内置公钥验签，delta 失败自动回退全量包；私钥丢失=已装应用搁浅，泄漏=可向用户推任意代码，须进 CI secrets）。

值得强调的机制事实：

- **更新签名与 OS 代码签名互相独立**：mygo 更新走自己的 Ed25519（install.sh 同样用 `openssl` 验 Ed25519），因此「无 Developer ID / 无 Windows 证书先发」**不阻塞自动更新**，升级包完整性与来源由 Ed25519 保证。来源：updates.md「Set up」、distribution.md「The install script」。
- delta 更新：构建时自动从已发布版本（默认近 3 个）生成 bsdiff 增量，manifest 列出，客户端按 size+SHA-256 逐文件校验重组，失败回退全量。来源：updates.md「Delta updates」。
- 版本号取自配置 `version`，release notes 取自 `CHANGELOG.md` 对应小节；prerelease 按 semver 比较，用户可在设置里开 beta 频道。来源：updates.md。

## 4. 推荐组合与成本表（问题④）

### 推荐

1. **分发通道（唯一）：GitHub Releases + 官方 release.yml**。`mygo build -upload` → draft → 人工过目发布；三平台产物 + `install.sh`/`install.ps1` 一行安装全挂 latest release。公开仓库 Releases/Actions 免费，零运维。不自建更新服务器、不用 Sparkle/WinSparkle 类第三方库——mygo.Updater 即 Sparkle 模式的官方实现（工程原则：不造轮子）。
2. **自动更新：`mygo keygen` + `mygo.Updater` + `plugins/updater`**，Day 1 即开。私钥进 GitHub Actions secrets；beta 频道留给 `SetPrereleases`。
3. **mac 签名：尽早买 Apple Developer Program（$99/年）**，配 `signingIdentity`+`notarize`，CI 脚本照抄官方文档。这是 mac 体验的硬门槛：不买则 DMG 用户人人两跳「仍要打开」。
4. **win 签名：首选申请 SignPath Foundation（$0）**——开源项目免费、证书绑定仓库、HSM+CI 集成，正对口 Remit；等批期间或批不下来再上 Certum Open Source（€25/年，需加密卡）。不买 EV（2024 起无即时豁免，纯浪费）；Azure Artifact Signing 对非美加个人不可用，仅在将来有合规组织主体时再评估。
5. **Linux 不需要任何签名**：tar.gz+install.sh / .deb / PKGBUILD 均为 mygo 原生产物，系统包路径交给包管理器。

### 成本表（年）

| 项 | 方案 | 成本 | 得到 |
|---|---|---|---|
| 分发托管 | GitHub Releases + Actions（公开仓库） | $0 | 三平台产物、一行安装、更新通道 |
| 自动更新 | mygo.Updater（Ed25519） | $0 | 全平台自更新+delta+beta 频道 |
| mac 签名+公证 | Apple Developer Program | **$99/年** | Gatekeeper 直接放行，无任何弹窗 |
| win 签名 | SignPath Foundation（申请制） | **$0**（批不下 → Certum €25/年 + 一次性卡） | 去掉 Unknown Publisher/蓝框（仍有 OV 级信誉积累期） |
| win 签名（不推荐） | EV 证书 | $400+/年 | 与 OV 无差别（SmartScreen），仅剩企业采购价值 |
| 备选 win 签名 | Azure Artifact Signing | ~$9.99/月 | 仅美加个人/美加EUUK组织可用，现阶段不适用 |

**最小可信年成本：$99（仅 Apple）+ €0~25（win 二选一）≈ $99–125/年；极限零成本路径也存在（见下）。**

### 「无签名先发」过渡路径的真实代价

可行（更新链路不受影响，Ed25519 独立验签），代价全在**首次安装体验**：

- mac（DMG 安装者）：Gatekeeper「Apple cannot check it for malicious software」→ 系统设置→隐私与安全性→仍要打开→再点一次「打开」，两跳；官方指引仅此一路。缓解：引导用 `install.sh`（curl 下载不设隔离属性，装完直接打开，无 Gatekeeper 提示——mygo 文档明言）。
- win：SmartScreen「Windows protected your PC」→ More info → Run anyway；UAC 显示 Unknown Publisher；企业策略可能彻底拦；Win11 Smart App Control 机器直接拒；**每个新版本从零积累哈希信誉**（签名版则按发布者证书继承）。缓解：引导用 `install.ps1`（校验 SHA256SUMS 后静默装，绕开浏览器下载链路）[推断：PowerShell 下载链路不写 Mark-of-the-Web，故少触发 SmartScreen；未在微软文档中明说]。
- 发布侧配套：README 写清两个「仍要打开/仍要运行」步骤+一行安装命令，即可把劝退率压到可接受。
- 结论：**无签名先发是可用的过渡态，不是死路**；但 mac 一旦有真实用户就值 $99/年——这是性价比最高的一笔。

### 与地图既定决策的一致性

开源产品、mygo 为 GUI/分发主体、明显更好/更成熟/更现代优先、不造轮子——推荐组合全部落在 mygo 内置能力 + GitHub 免费基础设施上，**无冲突事实**。唯一与「零成本理想」相悖的事实：mac 想要无弹窗体验没有免费官方路径（Apple 独家发证，$99/年）；win 有 SignPath/Certum 的开源免费/低价路径。
