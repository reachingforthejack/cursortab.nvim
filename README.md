# cursortab.nvim

A reverse engineered version of the [https://www.cursor.com/](Cursor) IDE's Tab completion API running in neovim.

This is extremely crude right now and barely implements any of the good features, like the diff history etc. but all of the API scaffolding is here.

Before going any further, I want to give thanks to [https://github.com/everestmz/cursor-rpc](this work) done by everestmz that saved me a lot of time and set up the scaffolding for locating these API nedpoints.

Tomorrow I'll update this with more detailed information on my findings, I'm very sleepy right now but wanted to get something up before the end of the day...

## How to install

Load this plugin using your favorite plugin manager. As a prerequisite, clone this repo and run `go install .`. If you have the `GOBIN` folder in your path, then you should be good. Eventually I'll ship a predistributed binary but for now this works.

If you have cursor installed, and you're signed into it this will just work automatically and pick up your login credentials

## Features

Currently implements a very, very basic version of the `CppCompletion` API for generating completions, as well as the `CursorPosition` API for jumps which doesn't work correctly right now.

`<Tab>` will trigger the completion you see to run. You cannot currently rebind this. I'm very new to neovim plugin dev, and I'm sure there's a much better way to set this up so I'd love any contributions.

## TODOs

- [ ] Diff the next keypress against the current buffer, so that we don't re-trigger a completion request if the user is "typing into" the completion on screen 
- [ ] Implement the diff history API
- [ ] Make the API of this be an actually good plugin API
- [ ] Make the queue of operations work correctly so the complete -> move cursor -> loop flow works correctly 
- [ ] Make context work 
- [ ] A million other things

## DISCLAIMER

If you get banned from Cursor for using this, I do not claim responsibility.
