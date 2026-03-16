from sqlalchemy import Column, Integer, String, Text, SmallInteger, DateTime, BIGINT, Index, Boolean, JSON
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.schema import UniqueConstraint
from datetime import datetime

Base = declarative_base()

# 1. 采集任务总表
class CrawlerTarget(Base):
    __tablename__ = "crawler_target"
    id = Column(BIGINT, primary_key=True, autoincrement=True)
    platform_type = Column(SmallInteger, nullable=False, default=3)
    # 归属大V的UID
    uid = Column(String(50), nullable=False, index=True)
    resource_type = Column(String(200), nullable=False, comment="资源维度:  scrape_and_store_video_comments, scrape_and_store_user_info...")
    # 具体的BV号、动态ID或UID等一众实体目标资源唯一ID
    target_id = Column(String(50), nullable=False)
    # 12小时
    cron_interval_minutes = Column(Integer, default=720)
    is_active = Column(Boolean, default=True)
    last_run_time = Column(DateTime, nullable=True)
    # 插入即刻触发
    next_run_time = Column(DateTime, default=datetime.now)
    create_time = Column(DateTime, default=datetime.now)
    update_time = Column(DateTime, default=datetime.now, onupdate=datetime.now)
    # 声明四元组复合唯一约束
    __table_args__ = (
        UniqueConstraint('platform_type', 'uid', 'resource_type', 'target_id', name='uk_target'),
    )

# 2. 采集任务快照表
class CrawlerTask(Base):
    """
    时序化异步采集任务表 (支持批量快照与调度)
    """
    __tablename__ = "crawler_task"
    id = Column(BIGINT, primary_key=True, autoincrement=True, comment="物理主键")
    # task_id 同时作为 ClickHouse 数据表中的 batch_id (批次追踪号)
    task_id = Column(String(50), unique=True, nullable=False, index=True, comment="全局唯一批次/任务号")
    # task_name字段在引入任务总表后可废弃
    task_name = Column(String(100), nullable=True, comment="任务备注名 (如: '美妆区百大UP主每日快照')")
    platform_type = Column(SmallInteger, default=3, comment="平台: 3=B站, 1=抖音, 2=Tiktok")
    # task_type = Column(SmallInteger, nullable=False,comment="任务动作: 1=全量评论(时序), 2=基础画像(时序), 3=核心指标监测(时序)")
    # 将 target 拆分为 "维度" 和 "负载集合"
    resource_type = Column(String(200), nullable=False,comment="资源维度:  scrape_and_store_video_comments, scrape_and_store_user_info...")
    resource_payload = Column(JSON, nullable=False, comment="资源清单 (支持批量): {'ids': ['BV1xx', 'BV2xx']}")
    task_status = Column(SmallInteger, default=0, index=True, comment="状态: 0=待执行, 1=执行中, 2=成功, 3=失败")
    params = Column(JSON, nullable=True, comment="调度参数与限制: {'max_pages': 5, 'retry': 3}")
    error_msg = Column(Text, nullable=True, comment="失败异常堆栈")
    create_time = Column(DateTime, default=datetime.now, comment="批次创建时间")
    update_time = Column(DateTime, default=datetime.now, onupdate=datetime.now, comment="状态更新时间")


