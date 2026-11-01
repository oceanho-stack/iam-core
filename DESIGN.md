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
│   ├── user.go      # User
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
平台系统租户 "platform" ──▶ 平台角色/平台权限点（管理整个系统）
```

### 4.2 实体要点

| 实体 | 关键字段 | 说明 |
|---|---|---|
| **Tenant** | `ID/Type/Slug/Status/OwnerUserID/System/配额` | 隔离顶层容器；个人/企业；`System` 标记平台租户 |
| **User** | `ID/Username/Email/Phone/PasswordHash/Status` | 登录主体，与租户多对多 |
| **Membership** | `UserID/TenantID/RoleID/IsOwner/Status` | User↔Tenant 中间实体，携带租户内角色 |
| **Team** | `TenantID/Name/Slug/LeaderUserID` | 租户内分组，数据范围授权依据 |
| **Role** | `Scope(system/tenant)/TenantID/Name` | 平台内置 + 租户自定义 |
| **Policy** | `TenantID/RoleID/Action/Resource/Effect` | 角色↔权限点绑定，租户级 |

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

---

## 6. 平台管理（第二个管理平面，已定方向）

平台管理是**与租户管理垂直的独立平面**，不另起炉灶，**复用同一套建模**。

### 6.1 平台 = 特殊系统租户

```
租户（Tenant）
├── 业务租户（个人/企业）：普通隔离
└── 系统租户 "platform"（System=true，bootstrap 创建，不可删、不计费）
        ├── 成员：平台管理员（super_admin / platform_admin / support）
        ├── 角色：平台角色（复用 Role）
        └── 权限点：平台级命名空间（platform:*）
```

### 6.2 平台权限点与业务权限点隔离

```go
// 业务权限点（租户内）
p, admin, tenant:A, team, create
// 平台权限点（系统租户，跨租户特权）
p, super_admin, platform, tenant,     manage   // 管理所有租户
p, platform_admin, platform, tenant,  read     // 只读所有租户
```

**关键**：平台管理员的操作对象是**别的租户**（跨租户特权），靠 `tenant:manage` 平台权限点控制。业务角色永远拿不到 `platform:*`。

### 6.3 需定的机制

- **bootstrap**：首启创建 `platform` 租户 + 内置平台角色 + 绑定首个 super_admin。
- **跨租户操作**：平台管理员看/管所有业务租户，靠平台权限点 + "以租户视角操作"机制。
- **隔离红线**：平台平面与租户平面权限点命名空间分开；业务租户不能访问 `platform:*`。

---

## 7. 已确立的技术决策速览（承接 docs 知识库）

- **先抽工具后业务**：通用能力已进 go-libs（日志/配置/DB/ID/加密/缓存/HTTP/时间…），IAM 只组装（ADR-002）。
- **分层**：domain / store / repo / service / handler / policy（ADR-001 变体，用 store 解耦 DB）。
- **统一协议**：`Resp[T]`、code 分段、分页、Header 约定（ADR-003）。
- **GORM 数据库无关**：不写方言，测试用内存 sqlite（ADR-004）。
- **BaseRepo 泛型基类**：xdb 提供，IAM 的 repo 嵌入复用。

---

## 8. 落地顺序（建议）

1. **domain 模型**（已建骨架）
2. **Store 接口 + repo 实现**（连 xdb + BaseRepo）
3. **授权服务（authorize）**：租户隔离 + RBAC + 数据范围（权限前置，紧跟 domain）
4. **认证 + token**（登录/签发/校验中间件）
5. **service：注册 / 租户生命周期**（个人自动建租户；企业建租户→邀请）
6. **client + iam-server 壳**（共享场景，按需后置）

---

## 9. 开放问题（待沟通）

> 这是本阶段最需要的输入，逐项确认后再冻结实现。

### 9.1 权限与数据范围

- [ ] **数据范围层**第一版做多深？只做"租户隔离 + RBAC"，还是把"团队/数据范围"也纳入第一版？
- [ ] 数据范围用 Casbin 的哪种建模？（`resource+action` 精确策略 vs `org/team scope`）是否引入"行级权限"（如只能看自己创建的）？

### 9.2 平台管理

- [ ] **平台管理员是否同时是业务租户用户**？（推荐：同一套 User，靠 Membership 区分平面）确认。
- [ ] 平台内置角色清单定哪些？（super_admin / platform_admin / support / …）
- [ ] bootstrap 由谁触发？（种子脚本 / 首个注册用户 / 环境变量指定 super admin）

### 9.3 认证与 token

- [ ] token 类型与签发方：JWT？自签？共享场景下由 `iam-server` 统一签发，不共享场景自签？
- [ ] 刷新策略 / 过期 / 登出（token 撤销）怎么做？
- [ ] 是否首版就支持 OAuth / 第三方登录 / SSO？（用户明确提到过多种前端，考虑预留）

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
