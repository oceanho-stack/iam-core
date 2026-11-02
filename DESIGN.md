# IAM 设计文档（iam-core）

> 状态：**草案，持续演进**。本文档是 IAM 底座的沟通载体，随讨论迭代，尚未到实现冻结。
> 阅读对象：你（决策者）+ AI（vibe coding / 结对编码）。涉及本项目的一切讨论先在本文件对齐。

---

## 1. 项目定位

自研一套**多端共用**的 Go 多租户身份与租户底座（IAM）：

- **多租户**：租户（Tenant）作为隔离顶层容器
- **身份**：用户（User）登录、认证
- **组织/团队**：租户内 Team 分组
- **权限**：租户隔离 + RBAC + 数据范围（Casbin）

前端/客户端：产品官网、用户 Console、手机 App、CLI 共用同一套 API 与协议。

**技术栈**：后端 Go（Gin + GORM v2 + Casbin + Redis + PostgreSQL）；前端 Vue3 + Vben Admin（Console）/ Nuxt3（官网）。数据库无关（GORM 不绑定数据库特性）。

---

## 1.1 IAM 与 RBAC 的定位澄清（业界标准对照，已定）

**IAM ≠ RBAC**。IAM 是"身份 + 访问管理"的**完整系统（大容器）**，RBAC 只是其中"授权"（AuthZ）的一种实现模型。防止做偏——只做成 Casbin+租户=权限库，而非 IAM。

```
IAM（系统）
├── 身份生命周期：注册、登录、注销、密码、账户恢复
├── 认证 AuthN：你是谁？登录方式、token、会话、SSO/OAuth   ← 第一优先
├── 授权 AuthZ：你能做什么？
│       └── RBAC（基于角色） / ABAC（基于属性） / ACL      ← RBAC 只是其一
├── 身份治理：用户/角色管理、权限分配、审计
└── 多端对接：官网/Console/App/CLI 的认证入口
```

### 业界标准产品对照

| 产品 | 定位 | 认证（AuthN） | 授权（AuthZ） |
|---|---|---|---|
| Keycloak | 完整 IAM | ✅ OIDC/OAuth2/SAML | ✅ 授权服务，RBAC 是其模型之一 |
| Auth0 / Okta | 完整 IAM（SaaS） | ✅ 认证为主打 | ✅ RBAC/ABAC |
| Logto / Casdoor | IAM | ✅ 认证优先 | ✅ RBAC/ABAC |
| Casbin | **授权库，非 IAM** | ❌ 不做认证 | ✅ 只做 AuthZ |

**结论**：完整 IAM（Keycloak/Logto/Casdoor 均如此）= 认证 + 授权 + 身份管理。本项目核心价值在"**身份与认证**"（多端登录/token/SSO），RBAC/Casbin 是授权子模块的组件。

### 认证协议决策（已定）：走 OIDC 系

- 认证对外走 **OIDC（OpenID Connect，基于 OAuth 2.0）**——业界认证事实标准（Keycloak/Logto/Casdoor 均实现）。
- 签发/校验 JWT，支持多端；未来接第三方登录/SSO 顺畅。
- 授权模型：粗粒度用 OAuth 2.0 scopes，细粒度（行/列）由业务扩展（§5.2）——与业界切法一致。

---

## 2. 部署形态：库 + 壳双形态（已定）

IAM 不做成"必须独立部署的服务"，而是**同一个内核，两种形态**，靠**数据源注入**区分。

```
               iam-core（纯库，不绑定 DB / 部署形态）
                       │ 数据源注入（依赖注入）
          ┌────────────┴─────────────┐
   共享场景                      不共享场景
   A、B 连同一个 IAM 库            A 连自己的库
   （或都走 iam-server 壳）         B 连自己的库（互不相干）
```

### 2.1 两种形态

| 维度 | 共享场景（理解 A） | 不共享场景（理解 B） |
|---|---|---|
| 数据源 | 一个共享的 IAM 库（权威） | 各自独立的库 |
| 部署 | 建议起 `iam-server` 壳（HTTP API 单一权威） | 各自嵌入 `iam-core`，无需壳 |
| A/B 对接 | 客户端 SDK 调 IAM API / 或直连共享库 | 直接 `import iam-core` |
| 典型例子 | 同一品牌多应用，一个账号登录全产品线 | 卖给不同客户的独立产品，各管各的用户 |

### 2.2 双形态的设计根基（硬性约束）

1. **`iam-core` 不持有数据库**——所有 DB 经 `Store` 接口注入，谁用谁给连接。
2. **认证与授权逻辑全在库内**——共享模式只是包成服务壳对外，不共享模式进程内调用。
3. **token 校验中间件在库内**——两种形态共用同一套校验，只是 token 签发源不同（自签 vs iam-server 签发）。

> 待沟通：见 §9「开放问题」——共享库模式下是否强制走 `iam-server` 壳、token 由谁签发。

---

## 3. 代码结构（iam-core）

```
iam-core/
├── domain/          # 纯领域模型，禁止 import gorm/gin/casbin
│   ├── tenant.go    # Tenant（个人/企业，含 system 标记）
│   ├── user.go      # User（业务用户）
│   ├── platform_user.go # PlatformUser（平台账号，独立体系）
│   ├── membership.go# User↔Tenant 成员关系（带角色）
│   ├── team.go      # Team + TeamMembership
│   ├── role.go      # Role（system/tenant 作用域）
│   └── policy.go    # Policy + Permission
├── store/           # 数据源接口（注入点）
│   └── store.go     # Store 接口：Tenant/User/Membership/Team/Role/Policy/…
├── repo/            # 实现，连 xdb + BaseRepo
├── service/         # 业务逻辑：注册、登录、成员管理、授权
│   ├── auth.go      # 认证（登录/签发token/刷新）
│   ├── tenant.go    # 租户生命周期（注册自动建个人租户/企业建租户）
│   └── authorize.go # 授权（Casbin：租户隔离 + RBAC + 数据范围）
├── auth/            # token 签发与校验（JWT/自签），Gin middleware
└── client/          # 共享场景：对接 iam-server 的 HTTP 客户端 + 校验中间件
```

`iam-server`（薄壳，非核心）：把 `service` 包成 HTTP API，持有共享库，作为跨应用权威。

---

## 4. 领域模型（已建骨架）

> 已在 `domain/` 落地，纯结构，未绑 gorm/casbin。

### 4.1 核心实体关系

```
User ──(Membership，带角色)──▶ Tenant ──▶ Team（租户内）
User ──(TeamMembership)──▶ Team
Tenant ──(Role，租户级可自定义)──▶ Policy（角色↔权限点，租户级）
平台平面：PlatformUser ──▶ 平台角色 ──▶ 平台权限点（platform:*，管理整个系统）
```

### 4.2 实体要点

| 实体 | 关键字段 | 说明 |
|---|---|---|
| **Tenant** | `ID/Type/Slug/Status/OwnerUserID/System/配额` | 隔离顶层容器；个人/企业；`System` 标记平台租户 |
| **User** | `ID/Username/Email/Phone/PasswordHash/Status` | 业务用户，与租户多对多 |
| **PlatformUser** | `ID/Username/PasswordHash/RoleIDs/Status` | **平台账号，独立体系**，非业务 User |
| **Membership** | `UserID/TenantID/RoleID/IsOwner/Status` | User↔Tenant 中间实体，携带租户内角色 |
| **Team** | `TenantID/Name/Slug/LeaderUserID` | 租户内分组，数据范围授权依据 |
| **Role** | `Scope(system/tenant)/TenantID/Name` | 平台内置 + 租户自定义 |
| **Policy** | `TenantID/RoleID/Action/Resource/Effect` | 角色↔权限点绑定，租户级（平台走 platform:*） |

---

## 5. 权限建模（核心，已定方向）

多租户下权限是**三层叠加**：

| 层 | 回答 | 示例 |
|---|---|---|
| **租户隔离层** | 在哪个租户内操作 | 张三看不到"李四的公司"的数据 |
| **角色/权限层**（RBAC） | 在租户内能做什么 | 管理员能建团队，成员只能看 |
| **数据范围层** | 同角色能看哪些数据 | 销售只看自己负责的客户 |

### 5.1 已定的决策

1. **Casbin 域机制 + GORM 适配器**：策略带 `tenantID` 作 domain，一个 Casbin 实例、一张策略表，靠 domain 隔离租户。
2. **权限点（Permission）平台级**：能力清单全局一套（`create:team`、`delete:user`…）。
3. **策略分配（Policy）租户级**：每个租户给角色配什么权限各自独立。
4. **角色租户级可自定义**：平台提供内置 `owner/admin/member`，租户可自建角色并配权限。
5. **数据范围层**：靠 Team/org scope 建模，Casbin 支持但要专门设计（见开放问题）。

```
p, admin, tenant:A, team,     create   # 租户A admin 可建团队
p, member, tenant:A, team,    read
g, 张三, admin, tenant:A                 # 张三在租户A 是 admin
```

### 5.2 IAM 与业务的数据权限边界（关键定位，已定）

**通用 IAM 不深度实现行级 / 列级（字段级）权限**，否则就从 IAM 变成业务权限引擎，且无法通用。原因：

- IAM 不知道业务表结构（列是"客户表的手机号"，业务私有模型）。
- 列级过滤要么介入 SQL（危险+强耦合），要么剥 DTO 字段（要懂每个接口），都让 IAM 深度绑定某业务。
- 行级 × 列级 × 租户交织时复杂度指数上升，全做进 IAM 会变成巨型引擎。

**业界参照**：Keycloak / Logto / Casdoor 只做租户 + RBAC（资源/动作级），行/列级几乎全交给业务。

#### 边界切分

```
IAM（通用，做）：                        业务（各系统，做）：
├── 租户隔离                             ├── 行级过滤（只看自己团队的客户）
├── RBAC（谁能 create:team）             └── 列级过滤（普通成员看不到手机号）
└── 提供"判定上下文 + 统一策略判定框架"
    （用户是谁/在哪个租户/什么角色/哪些团队/哪些属性）
```

**IAM 负责"判定规则"，业务负责"把数据模型喂给判定"**。业务注入自己的数据范围规则。

#### 统一判定入口（策略框架）

```go
// IAM 提供统一判定入口，业务传入数据上下文；IAM 不替业务写 WHERE、不剥 DTO
result, err := authz.Check(ctx, CheckInput{
    UserID:   123,
    TenantID: 9,
    Action:   "read",
    Resource: "customer",
    // 业务把"这条数据属于谁"传进来，IAM 按策略判定
    Object:   Object{OwnerTeamID: 5, Columns: []string{"phone", "amount"}},
})
// 业务据此过滤行、剥离列；判定逻辑通过钩子/接口可扩展
```

#### 结论

IAM 定位为：**租户隔离 + RBAC + 统一策略判定框架（支持 ABAC / 行 / 列的可扩展钩子）**，不替每个业务实现行列过滤。行/列判定逻辑由业务在 IAM 提供的判定接口上扩展注入。

---

## 6. 平台管理（第二个管理平面，已定方向）

平台管理是**与租户管理垂直的独立平面**。

### 6.1 平台账号体系（已定：独立账号，不复用业务 User）

平台管理员**使用独立的平台账号体系**，与业务租户的用户（User）彻底分开——平台管理员不是业务 User，也没有 Membership 进业务租户。

```
平台平面（独立账号体系）
├── PlatformUser（平台账号）→ 独立实体，非业务 User
│       登录：独立入口 / 独立凭据
├── 平台角色（复用 Role 结构，Scope=system）
└── 平台权限点（platform:* 命名空间）

业务平面
└── User + Membership（业务用户，与平台账号无交集）
```

**隔离语义**：
- 平台账号与业务 User **不互通**——一个人若是平台管理员，须有平台账号；若是业务用户，须是业务 User。两者各自独立。
- 平台账号的角色/权限走 `platform:*` 命名空间；业务角色永远拿不到 `platform:*`，平台角色默认不进业务租户。

> 权衡（记录）：独立账号体系隔离更彻底、审计清晰；代价是平台管理与业务身份两套账号、两套登录入口。此前考虑过"平台管理员复用同一套 User 靠 Membership 区分平面"，经权衡决定**独立账号体系**。

### 6.2 权限系统共用（独立账号体系不影响，已定）

**账号体系独立 ≠ 权限系统分裂**。独立的是"身份主体"（User vs PlatformUser），共用的是一整套授权引擎与模型（Casbin + Role + Policy）。这样既隔离彻底，又避免写两套权限。

```
权限系统（一套，共用）
├── 授权引擎：Casbin（同一引擎，不区分平台/业务）
├── 授权模型：Role + Policy（同一套结构）
│        Role.Scope = system（平台角色）/ tenant（业务角色）
│        Policy.TenantID = platform（平台）/ 业务租户ID
└── 主体（subject）：谁在发起请求？
        业务请求 → User
        平台请求 → PlatformUser      ← 仅这层不同
```

**账号体系决定"主体是谁"，权限系统决定"这个主体能做什么"**。主体可多套，授权逻辑、Casbin 引擎、策略存储统一一套。

```go
// 业务：主体 User，域 = 业务租户
p, admin, tenant:A, team, create
g, user张三, admin, tenant:A
// 平台：主体 PlatformUser，域 = platform
p, super_admin, platform, tenant, manage
g, platformuser_oceanho, super_admin, platform
```

同一 Casbin 实例、同一张策略表、同一套 Role/Policy 结构——仅"域（domain）"与"主体"不同。平台与业务在授权模型上零分裂（与 Keycloak 的 realm 模型一致）。

### 6.3 平台权限点与业务权限点隔离

```go
// 业务权限点（租户内，主体是 User+角色）
p, admin, tenant:A, team, create
// 平台权限点（平台平面，主体是 PlatformUser+平台角色）
p, super_admin, platform, tenant,     manage   // 管理所有租户
p, platform_admin, platform, tenant,  read     // 只读所有租户
```

**关键**：平台管理员的操作对象是**别的租户**（跨租户特权），靠 `tenant:manage` 平台权限点控制。业务角色永远拿不到 `platform:*`。

### 6.4 需定的机制

- **bootstrap**：首启创建 `platform` 系统租户 + 平台内置角色 + 绑定首个平台超级管理员。
- **跨租户操作**：平台管理员看/管所有业务租户，靠平台权限点 + "以租户视角操作"机制。
- **隔离红线**：平台平面与租户平面账号体系、权限点命名空间全部分开；业务租户不能访问 `platform:*`。

---

## 7. 已确立的技术决策速览（承接 docs 知识库）

- **先抽工具后业务**：通用能力已进 go-libs（日志/配置/DB/ID/加密/缓存/HTTP/时间…），IAM 只组装（ADR-002）。
- **分层**：domain / store / repo / service / handler / policy（ADR-001 变体，用 store 解耦 DB）。
- **统一协议**：`Resp[T]`、code 分段、分页、Header 约定（ADR-003）。
- **GORM 数据库无关**：不写方言，测试用内存 sqlite（ADR-004）。
- **BaseRepo 泛型基类**：xdb 提供，IAM 的 repo 嵌入复用。

---

## 8. 落地顺序（建议）

> IAM 核心价值在身份与认证（§1.1），故认证（AuthN）作为第一优先推进；授权（AuthZ）紧跟。

1. **domain 模型**（已建骨架）
2. **Store 接口 + repo 实现**（连 xdb + BaseRepo）
3. **认证 + token**（登录/签发/校验中间件；OIDC 系，JWT）
4. **授权服务（authorize）**：租户隔离 + RBAC + 数据范围（策略框架，业务扩展行/列）
5. **service：注册 / 租户生命周期**（个人自动建租户；企业建租户→邀请）
6. **client + iam-server 壳**（共享场景，按需后置）

---

## 9. 开放问题（待沟通）

> 这是本阶段最需要的输入，逐项确认后再冻结实现。

### 9.1 数据权限落地细节

- [ ] IAM 的**统一判定入口（`CheckInput`）与钩子**的接口形态怎么设计？（业务如何扩展注入行/列判定）
- [ ] 数据范围第一版做到哪：只做"租户隔离 + RBAC"？还是纳入 Team 粒度？行/列统一走扩展框架（已定方向）。

### 9.2 平台管理

> 已定：平台走**独立账号体系**（§6.1），不复用业务 User。剩余待细化：

- [ ] 平台账号登录方式：独立账号密码？是否也支持 OIDC/SSO？
- [ ] 平台内置角色清单定哪些？（super_admin / platform_admin / support / …）
- [ ] bootstrap 由谁触发？（种子脚本 / 首个注册用户 / 环境变量指定 super admin）

### 9.3 认证与 token

> 已定：走 OIDC 系、JWT（§1.1）。剩余待细化：

- [ ] token 签发方细节：共享场景由 `iam-server` 统一签发，不共享场景自签（确认）。
- [ ] 刷新策略 / 过期 / 登出（token 撤销）怎么做？
- [ ] OIDC 具体流程：授权码 / 客户端凭证？首版支持哪些登录方式（账号密码 / 第三方）？

### 9.4 租户与数据共享

- [ ] 共享场景（理解 A）：**是否强制走 `iam-server` 壳**，还是允许 A/B 直连同一个共享库？
- [ ] 一个用户多租户时，**当前操作的租户上下文**怎么传递？（每次请求带 `tenantID`，还是 token 里含多个租户？）

### 9.5 实体细节

- [ ] Tenant 的 `slug` 唯一性约束、个人租户的配额默认值？
- [ ] User 的 Username/Email 唯一性、是否允许空 Email（纯 OAuth）？
- [ ] Membership 的 `IsOwner` 与 Role 的一致性由谁保证（service 层）？

### 9.6 交付与仓库

- [ ] `iam-core` 独立 git 仓库（`github.com/oceanho-stack/iam-core`），`iam-server` 是否同仓库子目录还是独立仓库？
- [ ] docs 知识库的"仓库清单"与"当前进度"随本设计同步更新。

---

## 10. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-11-02 | 初稿：定位/双形态/domain 骨架/权限/平台管理；列出开放问题 |
| 2026-11-02 | §5.2 新增：IAM 与业务的数据权限边界（IAM 做 RBAC+策略框架，行/列级判定业务扩展注入） |
| 2026-11-02 | §1.1 新增：IAM 与 RBAC 定位澄清（业界标准对照）+ 认证走 OIDC 系决策；§8 落地顺序认证前置；§9.3 收窄 |
| 2026-11-02 | §6 平台管理改为**独立账号体系**（PlatformUser 独立实体，不复用业务 User）；同步 §3/§4/§9.2 |
| 2026-11-02 | §6.2 新增：权限系统共用结论（账号体系独立不影响——Casbin+Role+Policy 统一，仅主体与命名空间不同） |
