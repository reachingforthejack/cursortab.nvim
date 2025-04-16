local chan
local ns_id = vim.api.nvim_create_namespace("cursortab")

vim.api.nvim_set_hl(ns_id, "cursortabhl", {
	ctermbg = "DarkRed",
	bg = "#553333",
	bold = false,
})

vim.api.nvim_set_hl(ns_id, "cursortabhl_addition", {
	ctermfg = "DarkGreen",
	fg = "#335533",
	bg = "#101010",
	bold = false,
})

vim.api.nvim_set_hl(ns_id, "cursortabhl_yellowish", {
	ctermfg = "DarkYellow",
	fg = "#333333",
	bg = "#101010",
	bold = false,
})

vim.api.nvim_set_hl_ns(ns_id)

local function ensure_job()
	if chan then
		return chan
	end
	chan = vim.fn.jobstart({ "connectrpc" }, { rpc = true })
	return chan
end

vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
	callback = function()
		vim.fn.rpcrequest(ensure_job(), "cursortab_sync", ns_id)
	end,
})

vim.keymap.set("i", "<Tab>", function()
	vim.fn.rpcrequest(ensure_job(), "cursortab_tab_key", ns_id)
end, { noremap = true, silent = true })
