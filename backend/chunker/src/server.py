from concurrent import futures
import grpc
import logging
import tempfile
import os

import contracts.chunker.v1.chunker_pb2 as chunker_pb2
import contracts.chunker.v1.chunker_pb2_grpc as chunker_pb2_grpc

from langchain_text_splitters import RecursiveCharacterTextSplitter
from langchain_community.document_loaders import (
    PyPDFLoader,
    Docx2txtLoader,
    TextLoader,
    UnstructuredMarkdownLoader
)

from grpc_reflection.v1alpha import reflection


# =========================
# Logging setup
# =========================
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)s | %(message)s"
)

logger = logging.getLogger("chunker")


def get_request_id(context: grpc.ServicerContext) -> str:
    """Extract x-request-id from metadata if present."""
    try:
        meta = dict(context.invocation_metadata())
        return meta.get("x-request-id", "")
    except Exception:
        return ""


# =========================
# Service
# =========================
class ChunkerServicer(chunker_pb2_grpc.ChunkerServiceServicer):

    def ExtractChunks(self, request_iterator, context):
        request_id = get_request_id(context)
        logger.info(f"Request received | x-request-id={request_id}")

        file_content = bytearray()
        file_name = ""
        chunk_size = 1000
        chunk_overlap = 200

        try:
            for req in request_iterator:
                file_content.extend(req.file_content)
                file_name = req.file_name
                chunk_size = req.chunk_size or 1000
                chunk_overlap = req.chunk_overlap or 200

            suffix = os.path.splitext(file_name)[1]

            with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as f:
                f.write(file_content)
                tmp_path = f.name

            try:
                if suffix == ".pdf":
                    loader = PyPDFLoader(tmp_path)
                elif suffix == ".docx":
                    loader = Docx2txtLoader(tmp_path)
                elif suffix == ".md":
                    loader = UnstructuredMarkdownLoader(tmp_path)
                elif suffix == ".txt":
                    loader = UnstructuredMarkdownLoader(tmp_path)
                else:
                    loader = TextLoader(tmp_path)

                docs = loader.load()
                text = "\n".join([doc.page_content for doc in docs])

                splitter = RecursiveCharacterTextSplitter(
                    chunk_size=chunk_size,
                    chunk_overlap=chunk_overlap,
                    separators=["\n\n", "\n", ". ", " ", ""]
                )

                chunks = splitter.split_text(text)

                for i, chunk in enumerate(chunks):
                    yield chunker_pb2.ChunkResponse(
                        content=chunk,
                        index=i,
                        source=f"{file_name}:chunk{i}"
                    )

                logger.info(
                    f"Request completed successfully | "
                    f"x-request-id={request_id} | chunks={len(chunks)}"
                )

            finally:
                os.unlink(tmp_path)

        except Exception as e:
            logger.exception(
                f"Request failed | x-request-id={request_id} | error={str(e)}"
            )
            raise


# =========================
# Server
# =========================
def serve():
    logger.info("Application is starting...")

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=4))

    chunker_pb2_grpc.add_ChunkerServiceServicer_to_server(
        ChunkerServicer(),
        server
    )

    SERVICE_NAMES = (
        chunker_pb2.DESCRIPTOR.services_by_name['ChunkerService'].full_name,
        reflection.SERVICE_NAME,
    )

    reflection.enable_server_reflection(SERVICE_NAMES, server)

    server.add_insecure_port("[::]:50051")
    server.start()

    logger.info("Application started. Listening on :50051")

    server.wait_for_termination()

if __name__ == '__main__':
    serve()
