from sqlalchemy import Column, Integer, String, Text, SmallInteger, DateTime, BIGINT,Index
from sqlalchemy.ext.declarative import declarative_base
from datetime import datetime

Base = declarative_base()

# 1. 采集任务表 [2]
class CrawlerTask(Base):
    __tablename__ = "crawler_task"
    id = Column(BIGINT, primary_key=True, autoincrement=True, comment="任务唯一ID")
    user_id = Column(BIGINT, nullable=False, comment="关联用户中心user_base.ID")
    task_type = Column(SmallInteger, nullable=False, comment="任务类型：1=单视频，2=批量，3=红人数据")
    platform_type = Column(SmallInteger, nullable=False, comment="平台类型：1=抖音，2=TikTok，3=B站")
    input_content = Column(String(1000), nullable=False, comment="输入内容（链接/ID）")
    task_status = Column(SmallInteger, default=0, comment="状态：0=待执行，1=执行中，2=成功，3=失败")
    total_count = Column(Integer, default=0, comment="采集总数")
    success_count = Column(Integer, default=0, comment="成功数")
    create_time = Column(DateTime, default=datetime.now, comment="创建时间")
    update_time = Column(DateTime, default=datetime.now, onupdate=datetime.now, comment="更新时间")
    __table_args__ = (Index("idx_user_id", user_id),)

# 2. 采集记录表 [3]
class CrawlerRecord(Base):
    __tablename__ = "crawler_record"
    id = Column(BIGINT, primary_key=True)
    task_id = Column(BIGINT, nullable=False)
    resource_id = Column(String(50), comment="视频ID或红人ID")
    parsed_data = Column(Text, comment="标准化后的JSON数据") # 重点：存清洗后的数据
    create_time = Column(DateTime, default=datetime.now)

# bilibili数据采集结果表
class BilibiliRecord(Base):
    __tablename__ = "bilibili_record"
