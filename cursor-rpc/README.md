This is a fork of the fantastic work done by https://github.com/everestmz

His version of this was a little outdated, so I fixed up some things and then used the latest version of the RPC

# cursor-rpc

[Here's how I reverse engineered Cursor.](https://ev.dev/posts/reverse-engineering-cursor)

Protos and go library to communicate with Cursor's backend, without using Cursor. Useful if you're only allowed to run proprietary code through the Cursor servers/have a paid Cursor license, but want to use your own editor (Vim, Helix, etc) or build other features on top of Cursor.

Works by reverse-engineering the minified, obfuscated JS in Cursor's VSCode fork and acting like a bootleg module loader to stand up only the relevant modules needed to get the Protobuf typing info.

**NOTE**: To use a Cursor account with Vim or Helix, check out [sage](https://github.com/everestmz/sage).

## Usage

### Generating client libraries

Use the .proto files in `./cursor/aiserver/v1` to generate an RPC client library for your language.

### Go library

For detailed usage, check out [the basic example](cmd/example/main.go).

```go
// Get default credentials for Cursor:
// NOTE: you will need to open Cursor and log in at least once. You may need to re-login to
// refresh these credentials, or use the RefreshToken to get a new AccessToken.
creds, err := cursor.GetDefaultCredentials()
if err != nil {
	log.Fatal(err)
}

// Set up a service:
aiService := cursor.NewAiServiceClient()

// Get completions!
model := "gpt-4"
resp, err := aiService.StreamChat(context.TODO(), cursor.NewRequest(creds, &aiserverv1.GetChatRequest{
	ModelDetails: &aiserverv1.ModelDetails{
		ModelName: &model,
	},
	Conversation: []*aiserverv1.ConversationMessage{
		{
			Text: "Hello, who are you?",
			Type: aiserverv1.ConversationMessage_MESSAGE_TYPE_HUMAN,
		},
	},
}))
if err != nil {
	log.Fatal(err)
}

for resp.Receive() {
	next := resp.Msg()
	fmt.Printf(next.Text)
}
```

## Updating the schema

To update the schema, run

```console
make extract-schema
```

This command assumes you have a `Cursor.app` in your `/Applications` directory.

Then, run

```console
make generate
```

To use `buf` to generate updated Go bindings for cursor-rpc.


## Todos

- auto-generate the `cursor_version.txt` (currently manually set)
- auto-updating when Cursor has new releases
- more testing & examples
