from pydantic import BaseModel


class CookieUpdatePayload(BaseModel):
    cookie: str
    timestamp: str
    test: bool = False
    message: str = ""