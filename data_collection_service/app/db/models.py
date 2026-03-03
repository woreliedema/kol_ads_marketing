from sqlalchemy import Column, Integer, String, Text, SmallInteger, DateTime, BIGINT, Index, JSON
from sqlalchemy.ext.declarative import declarative_base
from datetime import datetime

Base = declarative_base()

# 1. 采集任务表 [2]
class CrawlerTask(Base):
    """
    时序化异步采集任务表 (支持批量快照与调度)
    """
    __tablename__ = "crawler_task"
    id = Column(BIGINT, primary_key=True, autoincrement=True, comment="物理主键")
    # 【核心重构 1】task_id 同时作为 ClickHouse 数据表中的 batch_id (批次追踪号)
    task_id = Column(String(50), unique=True, nullable=False, index=True, comment="全局唯一批次/任务号")
    task_name = Column(String(100), nullable=True, comment="任务备注名 (如: '美妆区百大UP主每日快照')")
    platform_type = Column(SmallInteger, default=3, comment="平台: 3=B站, 1=抖音, 2=Tiktok")
    # task_type = Column(SmallInteger, nullable=False,comment="任务动作: 1=全量评论(时序), 2=基础画像(时序), 3=核心指标监测(时序)")
    # 【核心重构 2】将 target 拆分为 "维度" 和 "负载集合"
    resource_type = Column(String(20), nullable=False,comment="资源维度:  scrape_and_store_video_comments, scrape_and_store_user_info...")
    resource_payload = Column(JSON, nullable=False, comment="资源清单 (支持批量): {'ids': ['BV1xx', 'BV2xx']}")
    task_status = Column(SmallInteger, default=0, index=True, comment="状态: 0=待执行, 1=执行中, 2=成功, 3=失败")
    params = Column(JSON, nullable=True, comment="调度参数与限制: {'max_pages': 5, 'retry': 3}")
    error_msg = Column(Text, nullable=True, comment="失败异常堆栈")
    create_time = Column(DateTime, default=datetime.now, comment="批次创建时间")
    update_time = Column(DateTime, default=datetime.now, onupdate=datetime.now, comment="状态更新时间")

# 2. 采集记录表 [3]
class CrawlerRecord(Base):
    __tablename__ = "crawler_record"
    id = Column(BIGINT, primary_key=True)
    task_id = Column(BIGINT, nullable=False)
    resource_id = Column(String(50), comment="视频ID或红人ID")
    parsed_data = Column(Text, comment="标准化后的JSON数据") # 重点：存清洗后的数据
    create_time = Column(DateTime, default=datetime.now)

