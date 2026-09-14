from dataclasses import dataclass
from .database import Base
from sqlalchemy import Column, Integer, String


class User(Base):
    __tablename__ = 'py_users'

    id = Column(Integer, primary_key=True)
    name = Column(String, nullable=False)


@dataclass
class UserDTO:
    id: int
    name: str
