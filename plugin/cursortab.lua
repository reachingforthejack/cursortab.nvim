local command = "CursorTab"

if vim.g.loaded_cursortab then
	return
end
vim.g.loaded_cursortab = true

vim.cmd [[
    highlight! default MyPreviewReplace ctermfg=Red cterm=bold,strikethrough gui=bold,strikethrough guifg=#FF0000
    highlight! default MyPreviewReplaceEOL ctermfg=Green cterm=bold gui=bold guifg=#00FF00
]]

vim.api.nvim_create_user_command(command, function(args)
	vim.rpcrequest(require('cursortab').get_chan(), "cursortab", args.fargs)
end, { nargs = "*" })

vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
	callback = vim.cmd[command],
})

vim.keymap.set("i", "Plug(cursor-tab)", function()
	vim.rpcrequest(require('cursor-tab').get_chan(), "cursortab_apply")
end, { silent = true })

if not vim.fn.hasmapto("Plug(cursor-tab)") then
	vim.keymap.set("i", "<Tab>", "Plug(cursor-tab)")
end
