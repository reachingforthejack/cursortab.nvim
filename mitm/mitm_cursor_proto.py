"""
This script was created by LaiKash.
The original version is available at https://github.com/LaiKash/cursor-aiserver-interceptor/blob/04dfa17212e35efd32e8d418f7253cabf2282160/mitm_cursor_proto.py
"""

import gzip
import os
from datetime import datetime

from mitmproxy import ctx, http
from mitmproxy.io import FlowWriter

import aiserver_pb2  # Make sure your compiled protobuf file is available

# Directory for log output
RESULTS_DIR = "results"
os.makedirs(RESULTS_DIR, exist_ok=True)

# Global list to store flows
captured_flows = []


def log_to_file(filename: str, content: str):
    path = os.path.join(RESULTS_DIR, filename)
    with open(path, "a", encoding="utf-8") as f:
        f.write(content + "\n" + ("-" * 80) + "\n")


def get_timestamp():
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")


def get_grpc_method_name(path: str) -> str:
    """
    Extracts the last part of the URL path.
    Example: "/aiserver.v1.AiService/StreamCpp" returns "StreamCpp"
    """
    return path.strip("/").split("/")[-1]


def get_proto_class(method: str, direction: str):
    """
    Returns the protobuf class for a given method.
    direction is 'request' or 'response'.
    For example, for method "StreamCpp", it returns aiserver_pb2.StreamCppRequest or
    aiserver_pb2.StreamCppResponse.
    """
    class_name = f"{method}{'Request' if direction == 'request' else 'Response'}"
    return getattr(aiserver_pb2, class_name, None)


def parse_grpc_messages(data: bytes, method: str, direction: str):
    """
    Iterates over the provided data buffer, extracting complete gRPC frames.
    Returns a tuple (messages, remaining_bytes), where messages is a list of decoded
    protobuf objects and remaining_bytes contains any leftover bytes (if the last frame
    was incomplete).
    """
    messages = []
    offset = 0
    total = len(data)
    while offset + 5 <= total:
        # Read header: 1-byte flag and 4-byte message length.
        compressed_flag = data[offset]
        msg_len = int.from_bytes(data[offset + 1 : offset + 5], byteorder="big")
        if offset + 5 + msg_len > total:
            ctx.log.warn("[!] Incomplete gRPC payload detected.")
            break
        message_bytes = data[offset + 5 : offset + 5 + msg_len]
        offset += 5 + msg_len

        # If the compressed flag is 1, decompress the message.
        if compressed_flag == 1:
            try:
                message_bytes = gzip.decompress(message_bytes)
            except Exception as e:
                ctx.log.warn(f"[!] Gzip decompression failed: {e}")
                continue

        msg_class = get_proto_class(method, direction)
        if not msg_class:
            ctx.log.warn(
                f"[!] Unknown protobuf class for method '{method}' ({direction})."
            )
            continue

        try:
            msg = msg_class()
            msg.ParseFromString(message_bytes)
            messages.append(msg)
        except Exception as e:
            ctx.log.warn(f"[!] Failed to parse protobuf message: {e}")
    remaining = data[offset:]
    return messages, remaining


def request(flow: http.HTTPFlow):
    # Only process if the URL contains the target service.
    if "aiserver.v1.AiService" not in flow.request.url:
        return

    # Also check for expected content type.
    content_type = flow.request.headers.get("content-type", "")
    if (
        "application/proto" not in content_type
        and "application/connect+proto" not in content_type
    ):
        return

    # Add flow to the list if it's relevant
    if flow not in captured_flows:
        captured_flows.append(flow)

    method = get_grpc_method_name(flow.request.path)
    ctx.log.info(f"[*] Detected gRPC request for method: {method}")
    try:
        raw = flow.request.raw_content
        if raw is not None:
            messages, remaining = parse_grpc_messages(raw, method, direction="request")
            if remaining:
                ctx.log.warn(
                    f"[!] Unparsed {len(remaining)} bytes remain in the request payload."
                )
        for msg in messages:
            decoded = str(msg)
            timestamp = get_timestamp()
            ctx.log.info(f"[gRPC Request {method}] Decoded:")
            ctx.log.info(decoded)
            log_to_file(
                "grpc_requests.log",
                f"[{timestamp}] Path: {flow.request.path}\n{decoded}",
            )
    except Exception as e:
        ctx.log.error(f"[!] Error parsing gRPC request: {e}")


def response(flow: http.HTTPFlow):
    # Only process if the URL contains the target service.
    if "aiserver.v1.AiService" not in flow.request.url:
        return

    # Ensure there is a response before proceeding
    if not flow.response:
        return

    # Check for expected content type.
    content_type = flow.response.headers.get("content-type", "")
    if (
        "application/proto" not in content_type
        and "application/connect+proto" not in content_type
    ):
        return

    # Add flow to the list if it's relevant and not already added by request
    if flow not in captured_flows:
        captured_flows.append(flow)

    method = get_grpc_method_name(flow.request.path)
    ctx.log.info(f"[*] Detected gRPC response for method: {method}")
    try:
        # Since we're not enabling streaming, we expect the full response.
        raw = flow.response.content
        if raw is not None:
            messages, remaining = parse_grpc_messages(raw, method, direction="response")
            if remaining:
                ctx.log.warn(
                    f"[!] Unparsed {len(remaining)} bytes remain in the response payload."
                )
            for msg in messages:
                decoded = str(msg)
                timestamp = get_timestamp()
                ctx.log.info(f"[gRPC Response {method}] Decoded:")
                ctx.log.info(decoded)
                log_to_file(
                    "grpc_responses.log",
                    f"[{timestamp}] Path: {flow.request.path}\n{decoded}",
                )
        else:
            ctx.log.warn(f"[!] Empty response content for method: {method}")
    except Exception as e:
        ctx.log.error(f"[!] Error parsing gRPC response: {e}")


def done():
    """
    Called when mitmproxy is shutting down. Saves captured flows to a .mitm file.
    """
    if not captured_flows:
        ctx.log.info("No relevant flows captured to save.")
        return

    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    filename = f"grpc_capture_{timestamp}.mitm"
    filepath = os.path.join(RESULTS_DIR, filename)

    try:
        with open(filepath, "wb") as f:
            fw = FlowWriter(f)
            for flow in captured_flows:
                fw.add(flow)
        ctx.log.info(f"[*] Saved {len(captured_flows)} flows to {filepath}")
    except Exception as e:
        ctx.log.error(f"[!] Failed to save flows to {filepath}: {e}")
