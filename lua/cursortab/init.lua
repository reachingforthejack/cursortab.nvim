local M = {}
local default_config = {
	sign_group = "CursortabGrp",
	sign_name = "Cursortab",
	sign_text = ">",
	sign_hl = "CursorLine",
	sign_priority = 10,
	preview_replace_hl = "MyPreviewReplace",
	preview_replace_eol_hl = "MyPreviewReplaceEOL",
	preview_line_hl = "Comment",
	command_name = "Cursortab",
}

local chan
local preview_namespace = vim.api.nvim_create_namespace("PreviewLineNS")

local function ensure_job()
	if chan then
		return chan
	end
	chan = vim.fn.jobstart({ "cursortab.nvim" }, { rpc = true })
	vim.fn.rpcrequest(chan, "cursortab_init")
	return chan
end

function M.PreviewLineContent(line_num, col_num, text_to_preview)
	local bufnr = vim.api.nvim_get_current_buf()
	local line_content = vim.api.nvim_buf_get_lines(bufnr, line_num - 1, line_num, false)[1] or ""
	local line_length = vim.api.nvim_strwidth(line_content)

	if col_num > line_length then
		col_num = line_length
	end

	vim.api.nvim_buf_set_extmark(bufnr, preview_namespace, line_num - 1, col_num, {
		virt_text = { { text_to_preview, "Comment" } },
		virt_text_pos = "overlay",
		hl_mode = "combine",
	})
end

function M.PreviewReplaceContent(line_num, new_text)
	local bufnr = vim.api.nvim_get_current_buf()
	vim.api.nvim_buf_add_highlight(bufnr, preview_namespace, M.config.preview_replace_hl, line_num - 1, 0, -1)
	vim.api.nvim_buf_set_extmark(bufnr, preview_namespace, line_num - 1, -1, {
		virt_text = { { new_text, M.config.preview_replace_eol_hl } },
		virt_text_pos = "eol",
	})
end

function M.ClearPreviewLineContent(startl, endl)
	local bufnr = vim.api.nvim_get_current_buf()
	vim.api.nvim_buf_clear_namespace(bufnr, preview_namespace, 0, -1)
end

function M.PlaceCursortabSign(cursor_line)
	local bufnr = vim.api.nvim_get_current_buf()
	vim.fn.sign_unplace(M.config.sign_group, { buffer = bufnr })
	vim.fn.sign_define(M.config.sign_name, {
		text = M.config.sign_text,
		texthl = M.config.sign_hl,
	})
	vim.fn.sign_place(0, M.config.sign_group, M.config.sign_name, bufnr, {
		lnum = cursor_line,
		priority = M.config.sign_priority,
	})
end

function M.ClearCursortabSign()
	local bufnr = vim.api.nvim_get_current_buf()
	vim.fn.sign_unplace(M.config.sign_group, { buffer = bufnr })
end

function M.setup(opts)
	M.config = vim.tbl_deep_extend("force", default_config, opts or {})
	vim.cmd([[
    highlight! default MyPreviewReplace ctermfg=Red cterm=bold,strikethrough gui=bold,strikethrough guifg=#FF0000
    highlight! default MyPreviewReplaceEOL ctermfg=Green cterm=bold gui=bold guifg=#00FF00
  ]])
	vim.api.nvim_create_user_command(M.config.command_name, function(args)
		vim.fn.rpcrequest(ensure_job(), "cursortab", args.fargs)
	end, { nargs = "*" })
	vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
		callback = function()
			vim.api.nvim_command(M.config.command_name)
		end,
	})
	vim.keymap.set("i", "<Tab>", function()
		vim.fn.rpcrequest(ensure_job(), "cursortab_apply")
	end, { noremap = true, silent = true })
end

return M
