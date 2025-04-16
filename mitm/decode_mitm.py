#!/usr/bin/env python
"""
A script to decode gRPC requests and responses for aiserver.v1.AiService
from a mitmproxy file and save the decoded payloads as JSON in a new mitmproxy file.
"""

import gzip
import json
from pathlib import Path
import sys  # Import sys module

from google.protobuf.json_format import MessageToDict
from mitmproxy import http, io

import aiserver_pb2


def parse_grpc_messages(data: bytes, msg_class) -> list[dict]:
    """Parse gRPC messages from binary data using a specific protobuf class."""
    messages = []
    offset = 0
    total = len(data)

    while offset + 5 <= total:
        # Read header: 1-byte flag and 4-byte message length
        compressed_flag = data[offset]
        msg_len = int.from_bytes(data[offset + 1 : offset + 5], byteorder="big")

        if offset + 5 + msg_len > total:
            print(f"Warning: Incomplete gRPC payload detected at offset {offset}")
            break

        message_bytes = data[offset + 5 : offset + 5 + msg_len]
        offset += 5 + msg_len

        # Handle compression if needed
        if compressed_flag == 1:
            try:
                message_bytes = gzip.decompress(message_bytes)
            except Exception as e:
                print(f"Error decompressing message: {e}")
                continue

        # Decode using the provided protobuf class
        try:
            msg = msg_class()
            msg.ParseFromString(message_bytes)
            # Use MessageToDict for dictionary output
            message_dict = MessageToDict(msg)
            messages.append(message_dict)
        except Exception as e:
            messages.append({"errror": str(e)})
            print(f"Error parsing protobuf message: {e}")

    return messages


def get_grpc_method_name(path: str) -> str | None:
    """
    Extracts the last part of the URL path if it matches the expected service format.
    Example: "/aiserver.v1.AiService/StreamCpp" returns "StreamCpp"
    Returns None if the path doesn't match the expected format.
    """
    parts = path.strip("/").split("/")
    if len(parts) == 2 and parts[0] == "aiserver.v1.AiService":
        return parts[1]
    return None


def get_proto_class(method: str, direction: str):
    """
    Returns the protobuf class for a given method and direction ('request' or 'response').
    Returns None if the class name doesn't exist in aiserver_pb2.
    """
    if not method:
        return None
    class_name = f"{method}{'Request' if direction == 'request' else 'Response'}"
    return getattr(aiserver_pb2, class_name, None)


def process_flow(flow: http.HTTPFlow) -> http.HTTPFlow:
    """Processes a gRPC flow, replacing binary content with JSON."""
    method = get_grpc_method_name(flow.request.path)
    if not method:
        print(
            f"Warning: Could not determine gRPC method from path: {flow.request.path}"
        )
        return flow  # Return unmodified flow if method unknown

    # Process request
    if flow.request and flow.request.raw_content:
        request_class = get_proto_class(method, "request")
        if request_class:
            messages = parse_grpc_messages(flow.request.raw_content, request_class)
            # Replace binary content with JSON
            flow.request.text = json.dumps({"messages": messages}, indent=2)
            flow.request.headers["content-type"] = "application/json"
        else:
            print(f"Warning: Unknown request class for method '{method}'")
            # Optionally keep original content or mark as undecoded
            flow.request.text = (
                f'{{"error": "Unknown request class for method \'{method}\'"}}'
            )
            flow.request.headers["content-type"] = "application/json"

    # Process response
    if flow.response and flow.response.content:
        response_class = get_proto_class(method, "response")
        if response_class:
            messages = parse_grpc_messages(flow.response.content, response_class)
            # Replace binary content with JSON
            flow.response.text = json.dumps({"messages": messages}, indent=2)
            flow.response.headers["content-type"] = "application/json"
        else:
            print(f"Warning: Unknown response class for method '{method}'")
            # Optionally keep original content or mark as undecoded
            flow.response.text = (
                f'{{"error": "Unknown response class for method \'{method}\'"}}'
            )
            flow.response.headers["content-type"] = "application/json"

    return flow


def main():
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <input_mitm_file>")
        sys.exit(1)

    input_mitmfile_path_str = sys.argv[1]
    input_mitmfile_path = Path(input_mitmfile_path_str)

    # Construct output path: replace .mitm with .decoded.mitm or append .decoded.mitm
    if input_mitmfile_path.suffix == ".mitm":
        output_mitmfile_path = input_mitmfile_path.with_suffix(".decoded.mitm")
    else:
        output_mitmfile_path = input_mitmfile_path.with_name(
            f"{input_mitmfile_path.name}.decoded.mitm"
        )

    print(f"Reading from: {input_mitmfile_path}")
    print(f"Writing decoded JSON to: {output_mitmfile_path}")

    try:
        with (
            open(input_mitmfile_path, "rb") as infile,
            open(output_mitmfile_path, "wb") as outfile,
        ):
            flow_reader = io.FlowReader(infile)
            flow_writer = io.FlowWriter(outfile)

            for flow in flow_reader.stream():
                if isinstance(flow, http.HTTPFlow):
                    # Only process and modify flows for the AiService
                    if "aiserver.v1.AiService/" in flow.request.url:
                        flow = process_flow(flow)

                # Write every flow (modified or not) to the output file
                flow_writer.add(flow)
        print("Processing complete.")
    except FileNotFoundError:
        print(f"Error: Input file not found at {input_mitmfile_path}")
        sys.exit(1) # Exit if file not found
    except Exception as e:
        print(f"An error occurred: {e}")
        sys.exit(1) # Exit on other errors


if __name__ == "__main__":
    main()
