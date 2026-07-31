## 文档基础信息
文档编号：CR-SYS-LLD-V1.0
前置依据：CR-SYS-02 SRS需求说明书、CR-SYS-03 HLD概要设计说明书
适用标准：CMMI DEV V2.0 L3已定义级
项目名称：云原生智能化代码评审系统 CR-System
密级：内部受控
编制日期：2026-07-30

## 1 总则
### 1.1 设计目的
细化概要设计中的服务逻辑、业务时序、接口定义、数据表字段、消息主题、异常处理规则，明确每个服务内部模块划分、代码分层结构，约束protobuf接口、MQ消息结构体、缓存Key命名规则，作为编码、单元测试、联调排查的直接依据，保证多开发人员编码口径统一。
### 1.2 内部代码分层统一规范（所有Go微服务强制遵守）
1. api层：对外gRPC/http接口定义、参数校验、请求接收
2. service层：业务逻辑实现，事务控制、状态流转、组合调用其他服务
3. data层：数据访问层，封装PostgreSQL、Redis、MQ操作，禁止service直连数据源
4. bizadapter层：第三方外部接口适配封装（Git、Sonar、推送渠道）
5. pkg公共层：全局错误码、工具函数、链路追踪、日志封装，所有服务共用

### 1.3 全局统一约束
1. 全链路携带TraceId，由APISIX生成，贯穿网关、所有微服务、MQ消息、日志、Jaeger；
2. 所有外部第三方调用必须增加重试机制、超时控制、熔断降级；
3. 数据库操作优先使用事务，核心状态变更保证原子性；
4. 禁止跨服务直接访问数据库，同步调用使用gRPC，异步依赖RocketMQ。

## 2 核心业务完整时序详细设计（MR创建评审单全流程）
1. 研发人员在Git平台完成MR提交，点击跳转CR-System前端页面，携带MR唯一ID、授权Code
2. 前端发起创建评审单HTTP请求 → APISIX完成HTTPS解密、JWT令牌校验、接口限流校验
3. 网关转发协议，转为gRPC请求发送至cr-core服务api层
4. cr-core api完成参数校验，进入service层，首先通过gRPC调用iam-service完成两层权限校验：
   - 用户是否登录有效
   - 用户是否拥有该仓库创建评审权限
5. 权限校验通过，开启数据库事务，向评审主单表插入一条待评审单据，状态=待评审
6. 事务内发起**RocketMQ事务消息（Topic: review_code_fetch_topic）**，绑定本地事务执行状态
   - 本地单据入库成功 → MQ消息正式投递；入库失败 → 消息直接丢弃，避免脏消息
7. git-adapter服务监听该Topic，消费消息后进入bizadapter层调用对应Git平台接口，拉取本次MR全部变更文件、原始Diff文本
8. git-adapter对原始Diff做结构化解析，生成「文件路径-行号-变更类型」映射结构体
9. 结构化Diff数据写入PostgreSQL变更行临时表，同时写入Redis缓存，缓存Key：`diff:{mrId}:{commitHash}`，TTL=7天
10. 前端采用轮询+SSE长连接轮询Diff就绪状态，检测缓存数据生成完成后，拉取结构化数据交付Monaco编辑器渲染分栏代码
11. 评审人员在线对指定行新增评论，前端携带文件路径、行号、版本ID提交至cr-core，评论数据绑定三元组存入评论表
12. 评审完成提交结论：
    - 驳回：评审单状态修改为「驳回待修改」，发送MQ通知消息至message-push服务，推送驳回提醒
    - 通过：自动同步调用Sonar适配接口拉取扫描结果，校验门禁规则，致命/严重漏洞存在则驳回放行；校验通过，单据状态变为「复审通过」
13. 代码合并完成后，Git平台Webhook回调系统，cr-core接收回调将单据自动置为归档状态，发送完结事件MQ
14. quality-stat消费归档事件，异步抽取所有缺陷评论，写入缺陷台账，定时任务后续完成统计计算与报表生成

## 3 六大微服务内部详细设计
### 3.1 iam-service 身份权限租户服务
1. 内部模块：租户管理模块、账号同步模块、RBAC权限模块、Keycloak适配模块
2. 核心gRPC接口：
   - CheckDataPermission(租户ID,仓库ID,用户ID) 返回权限布尔值
   - SyncUserFromKeycloak() 定时同步组织架构账号
3. 数据表核心字段：租户表、用户租户关联表、角色表、权限资源表、用户角色关联表
4. 定时任务：每日凌晨自动同步Keycloak新增/离职账号，禁用离职人员系统权限

### 3.2 git-adapter 仓库适配服务
1. 内部模块：OAuth授权管理、多平台适配器、MR同步模块、接口限流模块、Diff解析模块
2. Redis限流Key规则：`git_limit:{platform}:{ip}:hour`，计数器超过阈值拒绝调用Git接口
3. 授权Token存储：加密存入数据库，后台定时任务自动刷新Token有效期，失效前提前轮换
4. 黑名单拦截：读取仓库黑名单配置，命中后拒绝创建评审单，返回业务错误码

### 3.3 cr-core 评审核心服务
1. 内部模块：评审单管理、状态流转控制器、评论管理、复审校验、Sonar门禁校验
2. 状态流转硬编码约束，不允许随意变更：
   待评审→评审中→驳回待修改→复审提交→复审通过→归档
3. 分布式锁使用场景：`lock:review_mr:{mrId}`，防止同一个MR重复创建多张评审单，锁超时30秒
4. 评论分表策略：comment_yyyyMM，按月自动生成数据表，代码自动适配分表路由，无需手动建表

### 3.4 message-push 消息推送服务
1. 内部模块：消息解析模块、多渠道适配器、用户消息偏好过滤模块
2. 支持适配器：站内信适配器、企业微信机器人适配器、SMTP邮件适配器
3. 消费MQ统一Topic：review_notice_topic，根据消息内部eventType区分：新建评审、驳回、复审完成、超时预警
4. 用户可关闭对应渠道推送，消息读取用户配置后再选择性发送，无效消息直接丢弃不推送

### 3.5 quality-stat 质量统计服务
1. 内部模块：缺陷归集模块、指标计算模块、报表渲染模块、ES日志检索模块
2. 实时消费Topic：review_finish_topic，实时新增缺陷记录；
3. 每日凌晨定时任务：统计个人完成率、模块缺陷占比，计算技术债务分值；
4. 报表生成后转为Excel/PDF，上传至MinIO对应租户桶，返回文件访问地址存入数据库

### 3.6 job-scheduler 定时任务服务
所有任务基于K8s CronJob+内部任务管理器，任务列表：
1. 每6小时：仓库增量同步任务
2. 每日01:00：PostgreSQL评审单据与MQ消息数据对账，修复少量不一致脏数据
3. 每日02:00：清理Redis过期无效Diff缓存、临时Key
4. 每日03:00：自动刷新全部Git授权Token
5. 每月首日：生成上月全租户质量月报

## 4 RocketMQ 消息详细设计（主题、消息类型、使用方式）
| Topic名称 | 消息类型 | 使用方案 | 消费服务 |
| ---- | ---- | ---- | ---- |
| review_code_fetch_topic | 事务消息 | 创建评审单同步落库成功才发送消息 | git-adapter |
| review_notice_topic | 普通消息、延迟消息 | 通知推送、到期预警（延迟等级对应1天提醒） | message-push |
| review_finish_topic | 普通消息 | 评审归档完结，统计缺陷 | quality-stat |
| token_refresh_topic | 定时触发消息 | Git授权Token刷新提醒 | git-adapter |
补充规则：
1. 消息重试次数：最多3次重试，失败转入死信队列，运维后台可人工重推处理；
2. 延迟消息仅用于评审到期提醒，不再额外部署定时任务中间件。

## 5 关键数据表详细设计（核心表关键字段）
### 5.1 review_main 评审主单表
review_id(主键UUID)、tenant_id、mr_id、git_platform、title、creator_uid、reviewer_uid、priority、deadline、status、sonar_pass_flag、create_time、archive_time
### 5.2 review_comment 代码评论分表
comment_id、review_id、file_path、line_num、commit_version、defect_level、content、reply_parent_id、create_uid、create_time
### 5.3 git_auth 授权绑定表
auth_id、tenant_id、platform、auth_user_id、access_token(加密存储)、refresh_token、token_expire_time、sync_status
### 5.4 defect_record 缺陷台账表
defect_id、review_id、comment_id、defect_level、module_name、is_fixed、fix_time、stat_month

## 6 缓存Key详细命名规范（Redis统一格式）
1. 仓库基础信息：`repo:info:{repoId}` 过期时间12小时
2. 用户权限缓存：`user:perm:{uid}:{tenantId}` 过期时间2小时
3. Diff解析数据：`diff:{mrId}:{commitHash}` 过期7天
4. 分布式业务锁：`lock:{businessType}:{唯一ID}` 锁超时30s
5. Git接口限流计数：`git_limit:{platform}:{sourceIp}` 按小时过期

## 7 异常处理与错误码详细设计
### 7.1 错误码分层
1. 系统级错误（10xxx）：数据库异常、MQ发送失败、服务调用超时
2. 业务错误（20xxx）：权限不足、单据状态不允许操作、Sonar门禁不通过
3. 第三方错误（30xxx）：Git接口调用失败、推送接口返回异常
### 7.2 异常处理策略
1. 业务异常：直接返回前端提示文案，记录info级别日志
2. 系统&第三方异常：打印error日志、携带TraceId，触发平台告警；第三方接口采用指数退避重试（1s、3s、5s），重试失败标记任务异常
3. 所有禁止抛出原生panic，全局拦截统一封装返回结构体

## 8 前端关键组件详细设计
1. Monaco编辑器封装组件：支持Diff双栏对比、行号定位、评论悬浮展示、超大文件虚拟滚动分页加载
2. 权限路由守卫：基于Keycloak返回角色动态生成侧边栏菜单，无权限路由禁止访问
3. SSE消息订阅：实时接收评审状态变更、新评论提醒，无需高频轮询减少后端压力

## 9 安全详细细则
1. K8s Secret存放数据库账号、MQ地址、密钥、Sonar账号，代码与配置文件绝不出现明文密钥
2. 所有POST/PUT接口开启XSS过滤，表单特殊字符转义
3. 数据库所有模糊查询参数使用预编译，彻底杜绝SQL注入
4. 用户密码、Token全部AES加密存储，数据库不可解密查看原文
5. 操作日志写入ES，记录操作人、IP、操作内容、前后数据变更，满足审计追溯

## 审批签署栏
详细设计编制人：__________
接口联调评审人：__________
设计基线审批人：__________
生效日期：2026-07-30
