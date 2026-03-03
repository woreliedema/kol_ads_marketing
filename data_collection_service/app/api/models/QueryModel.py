from pydantic import BaseModel, Field
from typing import Any, List, Union
from enum import Enum

class OperatorEnum(str, Enum):
    EQ = "eq"       # 等于 =
    NE = "ne"       # 不等于 !=
    GT = "gt"       # 大于 >
    LT = "lt"       # 小于 <
    GTE = "gte"     # 大于等于 >=
    LTE = "lte"     # 小于等于 <=
    LIKE = "like"   # 模糊匹配 LIKE
    IN = "in"       # 包含 IN
    BETWEEN = "between" # 范围

class FilterRule(BaseModel):
    field: str = Field(..., description="数据库字段名，如 fans_count")
    op: OperatorEnum = Field(..., description="操作符")
    value: Union[str, int, float, List[Any]] = Field(..., description="筛选值")

class ComplexSearchRequest(BaseModel):
    filters: List[FilterRule] = []
    page: int = 1
    page_size: int = 20
    order_by: str = "create_time DESC"