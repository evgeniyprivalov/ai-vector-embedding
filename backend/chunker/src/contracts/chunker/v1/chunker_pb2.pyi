from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from typing import ClassVar as _ClassVar, Optional as _Optional

DESCRIPTOR: _descriptor.FileDescriptor

class ChunkRequest(_message.Message):
    __slots__ = ("file_content", "file_name", "chunk_size", "chunk_overlap")
    FILE_CONTENT_FIELD_NUMBER: _ClassVar[int]
    FILE_NAME_FIELD_NUMBER: _ClassVar[int]
    CHUNK_SIZE_FIELD_NUMBER: _ClassVar[int]
    CHUNK_OVERLAP_FIELD_NUMBER: _ClassVar[int]
    file_content: bytes
    file_name: str
    chunk_size: int
    chunk_overlap: int
    def __init__(self, file_content: _Optional[bytes] = ..., file_name: _Optional[str] = ..., chunk_size: _Optional[int] = ..., chunk_overlap: _Optional[int] = ...) -> None: ...

class ChunkResponse(_message.Message):
    __slots__ = ("content", "index", "source")
    CONTENT_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    SOURCE_FIELD_NUMBER: _ClassVar[int]
    content: str
    index: int
    source: str
    def __init__(self, content: _Optional[str] = ..., index: _Optional[int] = ..., source: _Optional[str] = ...) -> None: ...
