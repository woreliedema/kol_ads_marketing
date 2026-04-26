import time
from typing import Optional

# 1. 定义纯粹的 Lua 脚本常量 (不包含任何平台硬编码)
_LUA_FETCH_COOKIE_SCRIPT = """
local platform = KEYS[1]
local current_time = tonumber(ARGV[1])
local base_cooldown = tonumber(ARGV[2])

local zset_key = "cookie_pool:" .. platform

-- 取出 Score <= 当前时间的第一个 browser_id (已冷却完毕)
local result = redis.call('ZRANGEBYSCORE', zset_key, '-inf', current_time, 'LIMIT', 0, 1)
if not result or #result == 0 then
    return nil
end

local browser_id = result[1]
local hash_key = "cookie_info:" .. platform .. ":" .. browser_id

-- 获取元数据计算动态冷却时间
local last_update = tonumber(redis.call('HGET', hash_key, 'last_update') or current_time)
local fail_count = tonumber(redis.call('HGET', hash_key, 'fail_count') or 0)

-- 计算衰减逻辑 (超过3天未更新，每增加1天冷却倍数增加0.5)
local age_days = (current_time - last_update) / 86400
local time_penalty = 0
if age_days > 3 then
    time_penalty = (age_days - 3) * 0.5
end

-- 实际冷却秒数
local multiplier = (1 + time_penalty) * math.pow(2, fail_count)
local actual_cooldown = base_cooldown * multiplier

-- 更新下一次可用时间
local next_available_time = current_time + actual_cooldown
redis.call('ZADD', zset_key, next_available_time, browser_id)

return browser_id
"""


class CookieScheduler:
    def __init__(self):
        # 预留给 register_script 返回的 Script 对象
        self._script = None

    async def get_optimal_browser_id(self, redis_pool, platform: str, base_cooldown: int) -> Optional[str]:
        """
        核心调度方法：获取当前最优的节点 Browser ID
        """
        # 懒加载：只在第一次调用时注册 Lua 脚本到 Redis，获取 SHA1 缓存
        if self._script is None:
            self._script = redis_pool.register_script(_LUA_FETCH_COOKIE_SCRIPT)

        now_ts = int(time.time())
        try:
            # 执行预加载的脚本，底层会自动使用 EVALSHA
            # keys 对应 Lua 里的 KEYS[1], args 对应 ARGV[1], ARGV[2]
            eval_res = await self._script(
                keys=[platform],
                args=[now_ts, base_cooldown],
                client=redis_pool  # 显式传入连接池执行
            )

            if eval_res:
                return eval_res.decode('utf-8') if isinstance(eval_res, bytes) else eval_res
            return None
        except Exception as e:
            print(f"⚠️ [CookieScheduler] 调度 Lua 脚本执行异常: {e}")
            return None

    async def report_failure(self, redis_pool, platform: str, browser_id: str):
        """
        公共的风控上报方法，供所有爬虫调用
        """
        if not browser_id or browser_id == "local_config":
            return

        hash_key = f"cookie_info:{platform}:{browser_id}"
        try:
            await redis_pool.hincrby(hash_key, "fail_count", 1)
        except Exception as e:
            print(f"⚠️ [CookieScheduler] 上报风控失败: {e}")


# 导出一个单例供全局使用
cookie_scheduler_mgr = CookieScheduler()