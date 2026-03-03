import time
import logging


class SnowflakeGenerator:
    # 纪元时间（2026-01-01）
    TWepoch = 1772294400000

    def __init__(self, datacenter_id, worker_id):
        self.datacenter_id = datacenter_id
        self.worker_id = worker_id
        self.sequence = 0
        self.last_timestamp = -1

        # 位数分配
        self.worker_id_bits = 5
        self.datacenter_id_bits = 5
        self.sequence_bits = 12

        self.max_worker_id = -1 ^ (-1 << self.worker_id_bits)
        self.max_datacenter_id = -1 ^ (-1 << self.datacenter_id_bits)

        self.worker_id_shift = self.sequence_bits
        self.datacenter_id_shift = self.sequence_bits + self.worker_id_bits
        self.timestamp_left_shift = self.sequence_bits + self.worker_id_bits + self.datacenter_id_bits
        self.sequence_mask = -1 ^ (-1 << self.sequence_bits)

    def _time_gen(self):
        return int(time.time() * 1000)

    def generate_id(self):
        timestamp = self._time_gen()

        if timestamp < self.last_timestamp:
            raise Exception("时钟回拨异常！")

        if timestamp == self.last_timestamp:
            self.sequence = (self.sequence + 1) & self.sequence_mask
            if self.sequence == 0:
                # 当前毫秒序列号用完，等待下一毫秒
                while timestamp <= self.last_timestamp:
                    timestamp = self._time_gen()
        else:
            self.sequence = 0

        self.last_timestamp = timestamp

        # 位运算拼接出 64 位整型纯数字 ID
        return ((timestamp - self.TWepoch) << self.timestamp_left_shift) | (self.datacenter_id << self.datacenter_id_shift) | (self.worker_id << self.worker_id_shift) | self.sequence


# 单例实例化，假设机器号都为1 (生产环境中通过环境变量注入 Pod IP 计算)
snowflake_gen = SnowflakeGenerator(datacenter_id=1, worker_id=1)