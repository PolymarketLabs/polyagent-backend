PolyAgent 后端系统设计与 API 规范

1. 系统架构综述

    PolyAgent 采用 “离散结算 + 非裁量执行” 架构，旨在解决预测市场（Polymarket）资金聚合与自动化执行的合规性与安全性问题。
    账户体系: 基于 SIWE (EIP-4361) 的地址鉴权，JWT 强绑定。
    资金逻辑: 资金驻留 Vault 合约，执行权委托给受约束的 EOA 钱包。
    非裁量权: 后端充当“风控网关”，仅允许符合规则的 Intent（交易意图）流转至执行端。

2. 数据库核心模型 (ER Design)
    2.1 用户与角色 (Users)
        id: 唯一标识
        address: 钱包地址 (Unique Index)
        role: INVESTOR (默认), MANAGER (基金经理)
        is_verified: 经理审核状态
        kyc_status: 可选，用于合规性扩展

    2.2 基金详情 (Funds)

        id: 基金 ID
        vault_address: 链上 Vault 合约地址
        execution_address: 对应的 Polymarket 执行 EOA 地址
        manager_id: 关联 Users.id
        strategy_config: JSON (包含允许交易的市场类别、最大滑点、止损线)
        current_nav: 最新结算净值
        aum_total: 资产管理总规模 (Vault + Exec Wallet + Position)

    2.3 交易意图 (Intents)

        id: UUID
        fund_id: 所属基金
        market_id: Polymarket 市场 ID
        side: BUY / SELL
        order_data: JSON (价格、数量、订单类型)
        status: PENDING (待处理), VALIDATING (风控中), EXECUTING (执行中), SUCCESS, FAILED
        tx_hash: Polymarket 成交后的交易哈希

3. 全量 API 接口规范

    3.1 认证模块 (Auth)
        接口                        方法            说明

        /api/v1/auth/nonce          GET         获取登录随机 Nonce，存入 Redis
        /api/v1/auth/login          POST        提交 SIWE 签名，验签并下发 JWT
        /api/v1/user/profile        GET         获取当前用户信息及角色
        /api/v1/user/apply-manager  POST        投资人申请成为基金经理


    3.2 投资人模块 (Investor)
        |接口                           方法       说明      
                               |
        |/api/v1/funds                  GET       基金列表（含持仓、AUM、收益率排序）|
        /api/v1/funds/:id               GET       基金详情（含 AI 生成的风险评价标签）
        /api/v1/investor/portfolio      GET       我的投资组合（持仓详情、累计损益）
        /api/v1/investor/history        GET       充值、赎回、分红的历史记录（聚合链上数据）
        /api/v1/investor/rankings       GET       投资人收益排行榜

    3.3 基金经理模块 (Manager)
        接口                        方法                说明

        /api/v1/manager/funds       POST        创建新基金（初始化元数据）
        /api/v1/manager/my-funds    GET         我管理的基金列表及健康度
        /api/v1/manager/ai-pick     GET         AI 选品建议：基于 Polymarket 热度与波动率推荐市场
        /api/v1/manager/intents     POST        提交交易意图：触发非裁量校验流程
        /api/v1/manager/intents     GET         历史意图执行状态追踪

4. 关键流程详细设计
    4.1 非裁量执行 (Non-Discretionary Execution)
        Intent 接收: 后端拦截器从 JWT 获取 auth_address。
        所有权检查: 确认 auth_address 是目标 fund_id 的合法经理。
        风控硬约束:
        检查 market_id 是否在白名单。
        检查该笔交易金额是否超过基金当前可用余额的 X%。
        检查滑点是否在 strategy_config 定义的范围内。
        异步分发: 通过消息队列将校验通过的 Intent 发送至 Execution Worker。

    4.2 AI 模块逻辑

        AI 评价基金: 后端定时任务提取基金的收益曲线、回撤和交易频率，调用 LLM 生成摘要（如：“该经理偏好高风险政治预测，近期胜率 65%，适合激进型投资者”）。
        AI 选品建议: 后端 RAG 插件实时检索 Polymarket API，筛选未结算资金量大且信息不对称明显的市场，供经理参考。

5. 安全约束

    JWT 绑定: 所有涉及资金意图的操作必须校验 JWT 中的地址与资源所有权。
    单次 Nonce: Redis 存储 Nonce，使用后立即作废，防止重放攻击。
    EOA 隔离: 每个基金对应独立的执行 EOA，私钥由 KMS 环境隔离管理。