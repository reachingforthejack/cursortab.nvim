local M = {}

M.config = {
	sign_group = "CursortabGrp",
	sign_name = "Cursortab",
	sign_text = ">",
	sign_hl = "CursorLine",
	sign_priority = 10,
	preview_replace_hl = "CursortabPreviewReplace",
	preview_replace_eol_hl = "CursortabPreviewReplaceEOL",
	preview_line_hl = "Comment",
}

local api = vim.api
local chan
local preview_namespace = api.nvim_create_namespace("cursortab_preview")

---@return integer
function M.get_chan()
	if chan then
		return chan
	end
	chan = vim.fn.jobstart({ "cursortab.nvim" }, { rpc = true })
	vim.rpcrequest(chan, "cursortab_init")
	return chan
end

---@param line_num integer
---@param col_num integer
---@param text_to_preview string
function M.preview_line_content(line_num, col_num, text_to_preview)
	local bufnr = api.nvim_get_current_buf()
	local line_content = api.nvim_buf_get_lines(bufnr, line_num - 1, line_num, false)[1] or ""
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

---@param line_num integer
---@param new_text string
function M.preview_replace_content(line_num, new_text)
	local bufnr = vim.api.nvim_get_current_buf()
	vim.api.nvim_buf_add_highlight(bufnr, preview_namespace, M.config.preview_replace_hl, line_num - 1, 0, -1)
	vim.api.nvim_buf_set_extmark(bufnr, preview_namespace, line_num - 1, -1, {
		virt_text = { { new_text, M.config.preview_replace_eol_hl } },
		virt_text_pos = "eol",
	})
end

function M.clear_preview_line_content()
	local bufnr = api.nvim_get_current_buf()
	api.nvim_buf_clear_namespace(bufnr, preview_namespace, 0, -1)
end

---@param cursor_line integer
function M.place_cursor_tab_sign(cursor_line)
	local bufnr = api.nvim_get_current_buf()
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

function M.clear_cursor_tab_sign()
	local bufnr = api.nvim_get_current_buf()
	vim.fn.sign_unplace(M.config.sign_group, { buffer = bufnr })
end

---@param opts table
---@TODO: Define a struct for opts
function M.setup(opts)
	M.config = vim.tbl_deep_extend("force", M.config, opts or {})
end

return M
