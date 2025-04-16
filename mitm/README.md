This is a minimal setup for intercepting and decoding Cursor's requests and responses.

## Setup

The following setup should be compatible with macOS, Linux, and Windows.

### Install / Uninstall

1. Install [`uv`](https://docs.astral.sh/uv/getting-started/installation/). `uv` is used for managing Python versions and virtual environments.
2. Clone this very repo and navigate inside the `mitm` directory. All the code related to using mitmproxy for intercepting reqs/reqs will be kept in the `mitm` directory (including the virtual environment where the required tools will be installed).
3. Create the virtual environment and install dependencies by running:

```
uv sync
```

This will download and install the required Python version, create a virtual environment (the directory called `.venv` in the `mitm` directory), and install the specific dependencies version.

4. Follow the installation instructions for the mitmproxy CA certificate on [official docs](https://docs.mitmproxy.org/stable/concepts-certificates/).

To uninstall the dependencies and delete the virtual environment, simply delete the directory `.venv`.

### Usage

In order to start to intercept req/res or to run other commands using Python tools, you have to activate the virtual environment (you may need to activate it every time you start a new terminal session).

The virtual environment can be "activated" to make its packages available:

- macOS and Linux:

```
source .venv/bin/activate
```

- Windows:

```
.venv\Scripts\activate
```

To exit a virtual environment, use the deactivate command:

- macOS, Linux, and Windows: `deactivate`

#### Generating pb2 and pb2_grpc files

In order to decode req/res using the `cursor-rpc/cursor/aiserver/v1/aiserver.proto` specification, we need to generate the pb2 and pb2_grpc files.

From the `mitm` directory with the virtual environment activated, run the following command:

- macOS and Linux:

```
python -m grpc_tools.protoc -I=../cursor-rpc/cursor/aiserver/v1 --python_out=. --grpc_python_out=. ../cursor-rpc/cursor/aiserver/v1/aiserver.proto
```

- Windows:

```
python -m grpc_tools.protoc -I=..\cursor-rpc\cursor\aiserver\v1 --python_out=. --grpc_python_out=. ..\cursor-rpc\cursor\aiserver\v1\aiserver.proto
```

This command will create the files `aiserver_pb2.py` and `aiserver_pb2_grpc.py`. These two files are excluded from version control and should be created every time `aiserver.proto` is changed.

#### Running proxy

The mitmproxy must be run in local mode, i.e., it will intercept all the application(s) traffic **without** the need to configure the specific application (Cursor in our case) to make use of a proxy.

To start the proxy in local mode and intercept only Cursor req/res, use:

```
mitmweb --mode local:Cursor -s mitm_cursor_proto.py
```

The option `-s` selects a Python script to be used as an add-on, i.e., a simple script that extends proxy functionalities by using hooks provided by mitmproxy.

- `mitm_cursor_proto.py`: intercepts, decodes using Protobuf definitions, and logs gRPC messages for the `aiserver.v1.AiService`, saving the captured network flows to a `.mitm` file upon shutdown.
- _add other script here ..._

#### Inspecting results

Results are stored in the `results` directory. The results file can be

- `.log files` which can be opened with a text editor
- `.mitm files` which can be opened with the command: `mitmweb -r example.mitm`
