-- ============================================================================
-- react-base-service 建库建表脚本（新环境全量初始化）
--
-- 说明：
-- 1. 本脚本整合基座服务运行所需的全部 19 张表（含各历史迁移的最终列状态）。
-- 2. 基座服务不含知识库与 Plan 模式，相关表/列已移除。
-- 3. tblLlmReactSession/tblLlmReactRun/tblLlmReactMessage 等表在源仓库中无
--    CREATE TABLE 存档（仅存于现网库），此处按 GORM 模型定义忠实重建；
--    如与现网库存在细微差异，以现网 SHOW CREATE TABLE 为准。
-- 4. 内容列使用 MEDIUMTEXT：单条上限 16MB，受 max_allowed_packet 约束。
-- ============================================================================

CREATE DATABASE IF NOT EXISTS `llm` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;
USE `llm`;

-- ---------------------------------------------------------------------------
-- 一、调用方与注册资源（系统提示词 / 工具 / Skill / API Key）
-- ---------------------------------------------------------------------------

-- Caller 注册表
CREATE TABLE IF NOT EXISTS `tblLlmCaller` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `caller_key`  VARCHAR(32)  NOT NULL COMMENT 'caller唯一标识',
    `name`        VARCHAR(128) NOT NULL COMMENT 'caller名称',
    `description` TEXT         NULL     COMMENT '描述',
    `platform`    VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '兼容现有业务的平台标识',
    `status`      TINYINT      NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=启用',
    `created_by`  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`  BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_caller_key` (`caller_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM调用方注册表';

-- 系统提示词表（按 callerKey+routeValues 前缀匹配，多条短路由→长路由拼接）
CREATE TABLE IF NOT EXISTS `tblLlmSystemPrompt` (
    `id`           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `caller_key`   VARCHAR(32)  NOT NULL COMMENT '所属caller',
    `route_values` TEXT         NULL COMMENT '路由路径(JSON数组)',
    `name`         VARCHAR(128) NOT NULL COMMENT '提示词名称',
    `content`      TEXT         NOT NULL COMMENT '提示词内容',
    `status`       TINYINT      NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=启用',
    `created_by`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    INDEX `idx_caller_route` (`caller_key`, `route_values`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM系统提示词表';

-- Tool 表（Business Tool：http 类型由后端代理执行，client 类型由前端执行）
CREATE TABLE IF NOT EXISTS `tblLlmTool` (
    `id`           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `tool_id`      VARCHAR(64)  NOT NULL COMMENT '工具唯一标识(UUID)',
    `name`         VARCHAR(128) NOT NULL COMMENT '工具名称(英文标识)',
    `description`  TEXT         NULL COMMENT '工具描述',
    `tool_type`    VARCHAR(32)  NOT NULL COMMENT '工具类型: http/client',
    `caller_key`   VARCHAR(32)  NOT NULL COMMENT '所属caller',
    `route_values` TEXT         NULL COMMENT '路由路径(JSON数组)',
    `config`       TEXT         NULL COMMENT '工具配置(JSON: url/method/headers/inputSchema/outputSchema/async/asyncHint/frontendHint等)',
    `status`       TINYINT      NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=启用',
    `created_by`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_tool_id` (`tool_id`),
    INDEX `idx_caller_route` (`caller_key`, `route_values`(255)),
    INDEX `idx_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM工具表';

-- 工具用户访问策略表（白名单）
CREATE TABLE IF NOT EXISTS `tblLlmToolUserPolicy` (
    `id`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `tool_id`         VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'tblLlmTool.tool_id',
    `white_user_list` TEXT COMMENT '允许使用工具的用户名JSON数组',
    `black_user_list` TEXT COMMENT '禁止使用工具的用户名JSON数组，预留字段',
    `created_by`      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`      VARCHAR(64) NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_tool_id` (`tool_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM工具用户访问策略表';

-- Skill 表（按摘要索引注入 system 前缀，完整说明经 get_skill 按需加载）
CREATE TABLE IF NOT EXISTS `tblLlmSkill` (
    `id`                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `skill_id`            VARCHAR(64)  NOT NULL COMMENT '技能唯一标识(UUID)',
    `name`                VARCHAR(128) NOT NULL COMMENT '技能名称',
    `description`         TEXT         NULL COMMENT '技能描述',
    `trigger_condition`   TEXT         NULL COMMENT '触发条件(纯文本)',
    `forbidden_condition` TEXT         NULL COMMENT '禁止触发条件(纯文本)',
    `execution_steps`     TEXT         NULL COMMENT '执行步骤(纯文本)',
    `business_context`    TEXT         NULL COMMENT '业务背景(纯文本)',
    `prompt_supplement`   TEXT         NULL COMMENT '提示词补充(含枚举映射、接口约束等)',
    `caller_key`          VARCHAR(32)  NOT NULL COMMENT '所属caller',
    `route_values`        TEXT         NULL COMMENT '路由路径(JSON数组)',
    `is_default`          TINYINT      NOT NULL DEFAULT 0 COMMENT '是否兜底skill: 0=否 1=是',
    `status`              TINYINT      NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=启用',
    `created_by`          VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`          VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`          BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_skill_id` (`skill_id`),
    INDEX `idx_caller_route` (`caller_key`, `route_values`(255)),
    INDEX `idx_caller_status` (`caller_key`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM技能表';

-- 子 Agent 定义表（注册类资源：主 Agent 经 delegate_agent 工具委派子任务，子 run 隔离执行）
-- description 是委派质量的生命线：建议包含「适用问题类型 + 不适用边界」两段
CREATE TABLE IF NOT EXISTS `tblLlmAgent` (
    `id`              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `agent_id`        VARCHAR(64)  NOT NULL COMMENT 'Agent唯一标识(agent_前缀UUID)',
    `agent_key`       VARCHAR(64)  NOT NULL COMMENT '工具名级标识(如 dba-agent, delegate_agent 入参)',
    `name`            VARCHAR(128) NOT NULL COMMENT '展示名',
    `description`     TEXT         NOT NULL COMMENT '给主LLM看的委派说明(何时派给谁,对应 when_to_use)',
    `caller_key`      VARCHAR(32)  NOT NULL COMMENT '所属caller(default=全caller可用)',
    `route_values`    VARCHAR(256) NOT NULL DEFAULT '[]' COMMENT '路由路径(JSON数组)',
    `system_prompt`   MEDIUMTEXT   NOT NULL COMMENT '子Agent系统提示词(正文)',
    `model_key`       VARCHAR(64)  DEFAULT NULL COMMENT '模型种类(空=继承父run当前模型)',
    `model_version`   VARCHAR(128) DEFAULT NULL COMMENT '模型版本(空=按model_key解析默认)',
    `tools_json`      VARCHAR(1024) NOT NULL DEFAULT '[]' COMMENT '业务工具名白名单(JSON数组,空=继承caller全部可见工具)',
    `skills_json`     VARCHAR(1024) NOT NULL DEFAULT '[]' COMMENT 'Skill名白名单(JSON数组,空=不注入skill索引)',
    `max_steps`       INT          NOT NULL DEFAULT 8 COMMENT '子run步数上限(默认小于主run)',
    `permission_mode` VARCHAR(16)  NOT NULL DEFAULT 'inherit' COMMENT '权限模式: inherit/auto/confirm/confirm_risky(P2生效)',
    `status`          TINYINT      NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=启用',
    `created_by`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`      VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_agent_id` (`agent_id`),
    UNIQUE KEY `uk_caller_agent` (`caller_key`, `agent_key`),
    INDEX `idx_caller_route` (`caller_key`, `route_values`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM子Agent定义表';

-- MCP 连接注册表（管理接口登记的 MCP 服务器；启动时拉起客户端并同步工具进 tblLlmTool）
CREATE TABLE IF NOT EXISTS `tblLlmMcpServer` (
    `id`                 BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `server_id`          VARCHAR(64)  NOT NULL COMMENT '连接唯一标识(UUID)',
    `name`               VARCHAR(32)  NOT NULL COMMENT '服务器名(工具注册表前缀,全局唯一)',
    `kind`               VARCHAR(16)  NOT NULL DEFAULT 'http' COMMENT '传输类型: http/http_sdk/repo',
    `endpoint`           VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'HTTP MCP 端点URL',
    `headers`            TEXT         NULL COMMENT '附加请求头(JSON, 如 Authorization)',
    `env`                TEXT         NULL COMMENT 'stdio 适配器环境变量(JSON, 仅白名单kind)',
    `timeout_ms`         INT          NOT NULL DEFAULT 30000 COMMENT '单次操作超时毫秒',
    `description`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '描述',
    `status`             TINYINT      NOT NULL DEFAULT 1 COMMENT '状态: 0=停用 1=启用',
    `last_check_status`  VARCHAR(16)  NOT NULL DEFAULT 'unknown' COMMENT '最近连接检测: connected/disconnected/unknown',
    `last_check_message` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近连接检测信息',
    `last_check_at`      DATETIME     NULL COMMENT '最近连接检测时间',
    `caller_key`         VARCHAR(32)  NOT NULL COMMENT '所属caller',
    `created_by`         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`         VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`         DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_server_id` (`server_id`),
    UNIQUE KEY `uk_name` (`name`),
    INDEX `idx_caller` (`caller_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP连接注册表';

-- MCP 连接与 caller 的绑定（一个连接的工具同步到每个绑定的 caller；连接表 caller_key 为属主，恒定生效）
CREATE TABLE IF NOT EXISTS `tblLlmMcpServerCaller` (
    `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `server_id`  VARCHAR(64) NOT NULL COMMENT 'tblLlmMcpServer.server_id',
    `caller_key` VARCHAR(32) NOT NULL COMMENT '绑定的caller（工具同步到该caller名下）',
    `status`     TINYINT     NOT NULL DEFAULT 1 COMMENT '状态: 0=停用 1=启用',
    `created_by` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '创建人',
    `created_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_server_caller` (`server_id`, `caller_key`),
    INDEX `idx_caller` (`caller_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP连接caller绑定表';

-- API Key 表（caller 级模型调用凭证，按 callerKey+routeValues 解析）
CREATE TABLE IF NOT EXISTS `tblLlmApiKey` (
    `id`           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `caller_key`   VARCHAR(32)  NOT NULL COMMENT '所属caller',
    `route_values` TEXT         NULL COMMENT '路由路径(JSON数组)',
    `name`         VARCHAR(128) NOT NULL COMMENT 'key名称',
    `api_key`      VARCHAR(512) NOT NULL COMMENT '密钥值',
    `status`       TINYINT      NOT NULL DEFAULT 1 COMMENT '状态: 0=禁用 1=启用',
    `created_by`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '创建人',
    `updated_by`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '更新人',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    INDEX `idx_caller_route` (`caller_key`, `route_values`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM API Key表';

-- ---------------------------------------------------------------------------
-- 二、模型与积分（用户自定义模型 / 月度积分）
-- ---------------------------------------------------------------------------

-- 用户自定义模型表（playground 配置的个人模型，modelHash 供 run 直接引用）
CREATE TABLE IF NOT EXISTS `tblLlmUserModel` (
    `id`                  BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `model_hash`          VARCHAR(128) NOT NULL COMMENT '模型唯一hash(run按此引用)',
    `user_name`           VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '归属用户',
    `model_name`          VARCHAR(128) NOT NULL COMMENT '展示名称',
    `model_key`           VARCHAR(64)  NOT NULL COMMENT '模型种类',
    `model_version`       VARCHAR(128) NOT NULL COMMENT '模型版本',
    `api_key`             VARCHAR(512) NOT NULL DEFAULT '' COMMENT '该模型专用API Key',
    `biz_scenes`          TEXT         NOT NULL COMMENT '适用场景(JSON数组)',
    `is_platform_default` TINYINT      NOT NULL DEFAULT 0 COMMENT '是否平台默认: 0=否 1=是',
    `created_at`          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`          DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    `deleted_at`          BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '删除标记(0=未删除)',
    UNIQUE KEY `uk_model_hash` (`model_hash`),
    INDEX `idx_user_name` (`user_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM用户自定义模型表';

-- 月度基础积分表（粒度：user_name × model_hash）
CREATE TABLE IF NOT EXISTS `tblLlmUserBaseCredits` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `user_name`   VARCHAR(64)  NOT NULL COMMENT '用户名',
    `model_hash`  VARCHAR(128) NOT NULL COMMENT '模型hash',
    `credits`     INT          NOT NULL DEFAULT 1000 COMMENT '剩余积分',
    `reset_month` VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '积分重置月份(YYYY-MM)',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_user_model` (`user_name`, `model_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM用户月度基础积分表';

-- 管理员赠送积分表（粒度：user_name × model_hash）
CREATE TABLE IF NOT EXISTS `tblLlmUserBonusCredits` (
    `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `user_name`  VARCHAR(64)  NOT NULL COMMENT '用户名',
    `model_hash` VARCHAR(128) NOT NULL COMMENT '模型hash',
    `credits`    INT          NOT NULL DEFAULT 0 COMMENT '剩余赠送积分',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_user_model` (`user_name`, `model_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='LLM用户赠送积分表';

-- ---------------------------------------------------------------------------
-- 三、ReAct 会话 / 运行 / 消息（核心三表）
-- ---------------------------------------------------------------------------

-- ReAct 会话表
CREATE TABLE IF NOT EXISTS `tblLlmReactSession` (
    `id`           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `session_id`   VARCHAR(64)  NOT NULL COMMENT '会话唯一标识(session_前缀UUID)',
    `user_name`    VARCHAR(64)  NOT NULL COMMENT '归属用户',
    `caller_key`   VARCHAR(32)  NOT NULL COMMENT '调用方标识',
    `route_values` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '路由路径(JSON数组序列化)',
    `session_type` VARCHAR(20)  NOT NULL DEFAULT 'chat' COMMENT '会话类型: chat',
    `title`        VARCHAR(255) NOT NULL DEFAULT '' COMMENT '会话标题(按首条输入生成)',
    `last_run_id`  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '最后一次run ID',
    `last_message` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '最后一次摘要(512 rune截断)',
    `state`        VARCHAR(16)  NOT NULL DEFAULT 'active' COMMENT '状态: active/archived/deleted',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_session_id` (`session_id`),
    INDEX `idx_caller_route_user_state` (`caller_key`, `user_name`, `state`, `updated_at`),
    INDEX `idx_session_state` (`session_id`, `state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ReAct会话表';

-- ReAct 运行表（一次用户提交对应一次执行实例）
CREATE TABLE IF NOT EXISTS `tblLlmReactRun` (
    `id`                        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `run_id`                    VARCHAR(64)  NOT NULL COMMENT 'run唯一标识(run_前缀UUID)',
    `session_id`                VARCHAR(64)  NOT NULL COMMENT '所属会话ID',
    `user_name`                 VARCHAR(64)  NOT NULL COMMENT '发起用户',
    `caller_key`                VARCHAR(32)  NOT NULL COMMENT '调用方标识',
    `route_values`              VARCHAR(1024) NOT NULL DEFAULT '' COMMENT '路由路径(JSON数组序列化)',
    `state`                     VARCHAR(32)  NOT NULL DEFAULT 'running' COMMENT '状态: running/waiting_client_message/cancelling/finished/error/cancelled/expired',
    `step_index`                INT          NOT NULL DEFAULT 0 COMMENT '当前推理轮次',
    `max_steps`                 INT          NOT NULL COMMENT '最大推理轮次',
    `model_key`                 VARCHAR(64)  DEFAULT NULL COMMENT '模型种类',
    `model_version`             VARCHAR(128) DEFAULT NULL COMMENT '模型版本',
    `api_key`                   VARCHAR(512) DEFAULT NULL COMMENT '解析后的API Key',
    `control_context_json`      MEDIUMTEXT   COMMENT '运行控制参数快照',
    `llm_context_json`          MEDIUMTEXT   COMMENT '业务上下文快照',
    `tool_index_snapshot_json`  MEDIUMTEXT   COMMENT '工具索引快照(JSON)',
    `active_tool_ids`           TEXT         COMMENT '已加载工具ID列表(JSON)',
    `active_tool_defs_json`     MEDIUMTEXT   COMMENT '已加载工具定义快照(JSON)',
    `skills_index_snapshot_json` MEDIUMTEXT  COMMENT 'Skill索引快照(JSON)',
    `loaded_skill_ids`          TEXT         COMMENT '已加载skill ID列表(JSON)',
    `pending_tool_use_ids`      TEXT         COMMENT '等待前端回填的client tool调用ID(JSON)',
    `todo_state_json`           TEXT         COMMENT 'todo状态(JSON)',
    `total_input_tokens`        INT          NOT NULL DEFAULT 0 COMMENT '累计输入token',
    `total_output_tokens`       INT          NOT NULL DEFAULT 0 COMMENT '累计输出token',
    `last_input_tokens`         INT          NOT NULL DEFAULT 0 COMMENT '最近一轮输入token',
    `last_output_tokens`        INT          NOT NULL DEFAULT 0 COMMENT '最近一轮输出token',
    `cache_read_tokens`         INT          NOT NULL DEFAULT 0 COMMENT '累计缓存读token',
    `cache_create_tokens`       INT          NOT NULL DEFAULT 0 COMMENT '累计缓存写token',
    `error_message`             TEXT         COMMENT '失败原因',
    `parent_run_id`             VARCHAR(64)  DEFAULT NULL COMMENT '父run ID(delegate_agent子run指向父,外层run为NULL)',
    `agent_path`                VARCHAR(256) DEFAULT NULL COMMENT 'Agent路径(如main/ops-agent,外层run为NULL)',
    `created_at`                DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`                DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_run_id` (`run_id`),
    INDEX `idx_session_state` (`session_id`, `state`),
    INDEX `idx_session_created` (`session_id`, `created_at`),
    INDEX `idx_parent_run` (`parent_run_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ReAct运行表';

-- 存量环境增量迁移（新环境由上方建表语句直接包含）：
-- ALTER TABLE `tblLlmReactRun`
--     ADD COLUMN `parent_run_id` VARCHAR(64) DEFAULT NULL COMMENT '父run ID(delegate_agent子run指向父,外层run为NULL)' AFTER `error_message`,
--     ADD COLUMN `agent_path`    VARCHAR(256) DEFAULT NULL COMMENT 'Agent路径(如main/ops-agent,外层run为NULL)' AFTER `parent_run_id`,
--     ADD INDEX `idx_parent_run` (`parent_run_id`);

-- ReAct 消息表（run 内 seq 递增；content_json 存 modelMessage 与 toolMeta）
CREATE TABLE IF NOT EXISTS `tblLlmReactMessage` (
    `id`            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `message_id`    VARCHAR(64)  NOT NULL COMMENT '消息唯一标识(msg_前缀UUID)',
    `run_id`        VARCHAR(64)  NOT NULL COMMENT '所属run ID',
    `session_id`    VARCHAR(64)  NOT NULL COMMENT '所属会话ID',
    `user_name`     VARCHAR(64)  NOT NULL COMMENT '归属用户',
    `caller_key`    VARCHAR(32)  NOT NULL COMMENT '调用方标识',
    `seq`           INT          NOT NULL COMMENT 'run内递增序号',
    `step_index`    INT          NOT NULL DEFAULT 0 COMMENT '推理轮次',
    `role`          VARCHAR(16)  NOT NULL COMMENT 'system/user/assistant',
    `message_type`  VARCHAR(32)  NOT NULL COMMENT 'react_user_input/react_assistant/react_assistant_partial/react_tool_result/react_compact_summary',
    `model_key`     VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '模型种类(assistant消息)',
    `model_version` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '模型版本(assistant消息)',
    `content_json`  MEDIUMTEXT   COMMENT '消息内容(modelMessage与toolMeta)',
    `created_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_message_id` (`message_id`),
    INDEX `idx_run_seq` (`run_id`, `seq`),
    INDEX `idx_session_created` (`session_id`, `created_at`, `seq`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ReAct消息表';

-- ---------------------------------------------------------------------------
-- 四、ReAct 周边能力（工具大结果 / 产物 / 异步任务 / 反馈 / 附件）
-- ---------------------------------------------------------------------------

-- ReAct 工具大结果表（resultRef 指向的完整结果，分片读取）
CREATE TABLE IF NOT EXISTS `tblLlmReactToolResult` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `result_ref`  VARCHAR(128) NOT NULL COMMENT '工具大结果引用ID',
    `session_id`  VARCHAR(128) NOT NULL COMMENT 'ReAct会话ID',
    `run_id`      VARCHAR(128) NOT NULL COMMENT 'ReAct运行ID',
    `tool_use_id` VARCHAR(128) NOT NULL COMMENT '工具调用ID',
    `tool_name`   VARCHAR(128) NOT NULL DEFAULT '' COMMENT '工具名称',
    `content`     MEDIUMTEXT   NOT NULL COMMENT '完整工具结果，单条由业务限制最大16MB',
    `size_bytes`  INT          NOT NULL DEFAULT 0 COMMENT '内容字节数',
    `expire_at`   DATETIME     DEFAULT NULL COMMENT '过期时间',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_result_ref` (`result_ref`),
    KEY `idx_run_id` (`run_id`),
    KEY `idx_session_id` (`session_id`),
    KEY `idx_expire_at` (`expire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ReAct工具大结果表';

-- ReAct Python 产物文件表（文件本体存 COS）
CREATE TABLE IF NOT EXISTS `tblLlmReactArtifact` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `artifact_id` VARCHAR(64)  NOT NULL COMMENT '产物ID，前端下载接口按此取用',
    `session_id`  VARCHAR(128) NOT NULL COMMENT 'ReAct会话ID',
    `run_id`      VARCHAR(128) NOT NULL COMMENT 'ReAct运行ID',
    `tool_use_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '工具调用ID',
    `user_name`   VARCHAR(128) NOT NULL DEFAULT '' COMMENT '产物归属用户',
    `caller_key`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '调用方标识',
    `file_name`   VARCHAR(255) NOT NULL DEFAULT '' COMMENT '文件名',
    `mime_type`   VARCHAR(128) NOT NULL DEFAULT '' COMMENT '文件MIME类型',
    `cos_key`     VARCHAR(512) NOT NULL COMMENT 'COS对象Key',
    `size_bytes`  INT          NOT NULL DEFAULT 0 COMMENT '文件字节数',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    UNIQUE KEY `uk_artifact_id` (`artifact_id`),
    KEY `idx_run_id` (`run_id`),
    KEY `idx_session_id` (`session_id`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ReAct Python 产物文件表';

-- ReAct 异步任务待办表（含 Provider 状态同步列，最终形态）
CREATE TABLE IF NOT EXISTS `tblLlmReactAsyncTask` (
    `id`               BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `session_id`       VARCHAR(128) NOT NULL COMMENT 'ReAct会话ID',
    `run_id`           VARCHAR(128) NOT NULL COMMENT '提交时的ReAct运行ID',
    `tool_use_id`      VARCHAR(128) NOT NULL COMMENT '提交工具调用ID',
    `tool_name`        VARCHAR(128) NOT NULL DEFAULT '' COMMENT '提交工具名称',
    `submit_input`     MEDIUMTEXT   COMMENT '提交入参快照,get_async_task 回读全量',
    `submit_result`    MEDIUMTEXT   COMMENT '提交响应快照,含任务id等句柄,get_async_task 回读全量',
    `async_hint`       VARCHAR(512) NOT NULL DEFAULT '' COMMENT '结果查询提示(工具config.asyncHint快照)',
    `scheduler_type`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '调度系统Provider类型',
    `task_key`         VARCHAR(128) NOT NULL DEFAULT '' COMMENT '调度系统内稳定任务关联键',
    `task_info`        JSON         NULL COMMENT 'Provider识别和查询任务所需的非敏感JSON信息',
    `state`            VARCHAR(16)  NOT NULL DEFAULT 'pending' COMMENT '本地处理状态: pending/resolved/expired',
    `execution_status` VARCHAR(32)  NOT NULL DEFAULT '' COMMENT '第三方规范化执行状态: processing/succeeded/failed，空表示旧模式',
    `provider_status`  VARCHAR(128) NOT NULL DEFAULT '' COMMENT '第三方原始状态',
    `progress`         INT          NOT NULL DEFAULT -1 COMMENT '第三方任务进度，-1表示未提供',
    `error_message`    TEXT         COMMENT '第三方任务失败信息',
    `last_event_id`    VARCHAR(256) NOT NULL DEFAULT '' COMMENT '最近处理的第三方事件ID',
    `last_sequence`    BIGINT       NOT NULL DEFAULT 0 COMMENT '最近处理的第三方事件序号',
    `last_observed_at` DATETIME     NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT '最近一次有效状态观测时间',
    `completed_at`     DATETIME     NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT '第三方任务进入终态时间',
    `next_sync_at`     DATETIME     NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT '下一次主动对账时间',
    `retry_count`      INT          NOT NULL DEFAULT 0 COMMENT '连续主动对账失败次数',
    `lease_owner`      VARCHAR(128) NOT NULL DEFAULT '' COMMENT '主动对账租约持有者',
    `lease_until`      DATETIME     NOT NULL DEFAULT '1970-01-01 00:00:00' COMMENT '主动对账租约到期时间',
    `last_sync_error`  TEXT         COMMENT '最近一次主动对账错误',
    `resolve_status`   VARCHAR(16)  NOT NULL DEFAULT '' COMMENT '本地处理结果: done/failed',
    `expire_at`        DATETIME     NOT NULL COMMENT '提醒过期时间,过期后不再注入',
    `created_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`       DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_tool_use_id` (`tool_use_id`),
    KEY `idx_session_state` (`session_id`, `state`, `expire_at`),
    KEY `idx_run_id` (`run_id`),
    KEY `idx_scheduler_task_state` (`scheduler_type`, `task_key`, `state`),
    KEY `idx_async_task_sync_due` (`scheduler_type`, `execution_status`, `next_sync_at`, `lease_until`),
    KEY `idx_session_pending_page` (`session_id`, `state`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ReAct异步任务待办表';

-- ReAct 轮次反馈表（按 run_id 唯一，点赞/点踩与问题反馈）
CREATE TABLE IF NOT EXISTS `tblLlmReactRunFeedback` (
    `id`                         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `run_id`                     VARCHAR(64)  NOT NULL COMMENT 'run ID',
    `session_id`                 VARCHAR(64)  NOT NULL COMMENT '会话ID',
    `user_name`                  VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '反馈用户',
    `feedback`                   TINYINT      NOT NULL DEFAULT 0 COMMENT '点赞状态: 1=赞 -1=踩 0=无',
    `problem_feedback`           VARCHAR(2048) NOT NULL DEFAULT '' COMMENT '问题反馈文本',
    `feedback_updated_at`        DATETIME     DEFAULT NULL COMMENT '点赞更新时间',
    `problem_feedback_updated_at` DATETIME    DEFAULT NULL COMMENT '问题反馈更新时间',
    `created_at`                 DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`                 DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_run_id` (`run_id`),
    KEY `idx_session_id` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='ReAct轮次反馈表';

-- 对话附件记录表（csv/md/txt 上传至 COS，fileId 供 run attachments 引用）
CREATE TABLE IF NOT EXISTS `tblLlmChatFileRecord` (
    `id`         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `file_id`    VARCHAR(64)  NOT NULL COMMENT '文件唯一标识(file_前缀)',
    `owner`      VARCHAR(64)  NOT NULL COMMENT '上传用户',
    `file_name`  VARCHAR(255) NOT NULL COMMENT '原始文件名',
    `ext`        VARCHAR(16)  NOT NULL COMMENT '扩展名: csv/md/txt',
    `mime_type`  VARCHAR(128) DEFAULT NULL COMMENT 'MIME类型',
    `charset`    VARCHAR(32)  DEFAULT NULL COMMENT '检测出的字符集',
    `size`       BIGINT       NOT NULL COMMENT '文件字节数',
    `cos_key`    VARCHAR(512) NOT NULL COMMENT 'COS对象Key',
    `cos_uri`    VARCHAR(1024) DEFAULT NULL COMMENT 'COS访问URI',
    `status`     VARCHAR(16)  NOT NULL COMMENT '状态: parsed/deleted/expired',
    `created_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_file_id` (`file_id`),
    KEY `idx_owner` (`owner`),
    KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='对话附件记录表';

-- ---------------------------------------------------------------------------
-- 五、长期记忆（跨会话记忆，双层：常驻 resident / 按需 detached）
-- ---------------------------------------------------------------------------

-- 长期记忆条目表（一条原子事实；逻辑唯一键 owner_type+owner_key+item_key，item_key 为内容语义指纹）
CREATE TABLE IF NOT EXISTS `tblLlmMemoryItem` (
    `id`           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `owner_type`   VARCHAR(32)  NOT NULL COMMENT '记忆归属维度: caller/caller_user',
    `owner_key`    VARCHAR(128) NOT NULL COMMENT '记忆空间键: callerKey 或 callerKey|userName',
    `layer`        VARCHAR(16)  NOT NULL DEFAULT 'detached' COMMENT '层级: resident=常驻(全文注入)/detached=按需(目录索引)',
    `title`        VARCHAR(128) NOT NULL DEFAULT '' COMMENT '短标题(目录索引展示,≤32字)',
    `content`      TEXT         NOT NULL COMMENT '记忆正文(一到三句原子事实,≤500字)',
    `description`  VARCHAR(512) NOT NULL DEFAULT '' COMMENT '检索描述: 什么场景需要想起这条记忆',
    `tags`         VARCHAR(512) NOT NULL DEFAULT '' COMMENT '逗号分隔标签(memory_list过滤)',
    `source`       VARCHAR(32)  NOT NULL DEFAULT 'model' COMMENT '写入来源: model/reflection/admin',
    `item_key`     VARCHAR(64)  NOT NULL COMMENT '内容语义指纹(规范化SHA-256前16位,幂等去重)',
    `version`      INT          NOT NULL DEFAULT 1 COMMENT '乐观锁版本号,每次修订+1',
    `state`        VARCHAR(16)  NOT NULL DEFAULT 'active' COMMENT '状态: active/deleted(软删)',
    `last_reason`  VARCHAR(512) NOT NULL DEFAULT '' COMMENT '最近一次修订原因(冗余展示)',
    `created_by`   VARCHAR(64)  NOT NULL DEFAULT '' COMMENT '触发写入的runID或操作人',
    `created_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    UNIQUE KEY `uk_owner_item` (`owner_type`, `owner_key`, `item_key`),
    INDEX `idx_owner_layer_state` (`owner_type`, `owner_key`, `layer`, `state`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='长期记忆条目表';

-- 长期记忆修订流水表（不可变只插不改；回滚=用旧快照反向提交新修订）
CREATE TABLE IF NOT EXISTS `tblLlmMemoryRevision` (
    `id`          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY COMMENT '自增主键ID',
    `item_id`     BIGINT UNSIGNED NOT NULL COMMENT '记忆条目ID',
    `action`      VARCHAR(16)  NOT NULL COMMENT '动作: create/update/delete/rollback',
    `before_json` MEDIUMTEXT   COMMENT '变更前快照(create时为空)',
    `after_json`  MEDIUMTEXT   COMMENT '变更后快照(delete时为空)',
    `reason`      VARCHAR(512) NOT NULL COMMENT '修订原因(必填,审计根)',
    `source`      VARCHAR(32)  NOT NULL COMMENT '来源: model/reflection/admin',
    `created_by`  VARCHAR(64)  NOT NULL COMMENT 'runID或操作人',
    `created_at`  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX `idx_item` (`item_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='长期记忆修订流水表';
