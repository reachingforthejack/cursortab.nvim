local chan

local function ensure_job()
	if chan then
		return chan
	end
	chan = vim.fn.jobstart({ "cursortab.nvim" }, { rpc = true })
	vim.fn.rpcrequest(ensure_job(), "cursortab_init")
	return chan
end

vim.api.nvim_create_user_command("Cursortab", function(args)
	vim.fn.rpcrequest(ensure_job(), "cursortab", args.fargs)
end, { nargs = "*" })

vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
	callback = function()
		vim.api.nvim_command("Cursortab")
	end,
})

local preview_namespace = vim.api.nvim_create_namespace("PreviewLineNS")

local preview_namespace = vim.api.nvim_create_namespace("PreviewLineNS")

local preview_namespace = vim.api.nvim_create_namespace("PreviewLineNS")

vim.cmd([[
  highlight! default MyPreviewReplace ctermfg=Red cterm=bold,strikethrough gui=bold,strikethrough guifg=#FF0000
  highlight! default MyPreviewReplaceEOL ctermfg=Green cterm=bold gui=bold guifg=#00FF00
]])

function PreviewLineContent(line_num, col_num, text_to_preview)
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
		right_gravity = false,
	})
end

function PreviewReplaceContent(line_num, new_text)
	local bufnr = vim.api.nvim_get_current_buf()

	vim.api.nvim_buf_add_highlight(bufnr, preview_namespace, "MyPreviewReplace", line_num - 1, 0, -1)

	vim.api.nvim_buf_set_extmark(bufnr, preview_namespace, line_num - 1, -1, {
		virt_text = { { new_text, "MyPreviewReplaceEOL" } },
		virt_text_pos = "eol",
	})
end

function ApplyPreviewLineContent(line_num)
	local bufnr = vim.api.nvim_get_current_buf()

	local extmarks = vim.api.nvim_buf_get_extmarks(bufnr, preview_namespace, line_num - 1, line_num, { details = true })

	if #extmarks == 0 then
		return
	end

	local mark = extmarks[1]
	local details = mark[4]
	local virt_text_chunks = details.virt_text
	if not virt_text_chunks then
		return
	end

	local combined_text = {}
	for _, chunk in ipairs(virt_text_chunks) do
		table.insert(combined_text, chunk[1])
	end

	if #combined_text > 0 then
		vim.api.nvim_buf_set_lines(bufnr, line_num - 1, line_num, false, {
			table.concat(combined_text, ""),
		})
	end
	vim.api.nvim_buf_clear_namespace(bufnr, preview_namespace, line_num - 1, line_num)
end

function ClearPreviewLineContent(line_num)
	local bufnr = vim.api.nvim_get_current_buf()

	vim.api.nvim_buf_clear_namespace(bufnr, preview_namespace, line_num - 1, -1)
end

function PlaceCursortabSign(cursor_line)
	local bufnr = vim.api.nvim_get_current_buf()

	vim.fn.sign_unplace("CursortabGrp", { buffer = bufnr })

	vim.fn.sign_define("Cursortab", {
		text = ">",
		texthl = "CursorLine",
	})

	vim.fn.sign_place(0, "CursortabGrp", "Cursortab", bufnr, { lnum = cursor_line, priority = 10 })
end

function ClearCursortabSign()
	local bufnr = vim.api.nvim_get_current_buf()
	vim.fn.sign_unplace("CursortabGrp", { buffer = bufnr })
end

vim.keymap.set("i", "<Tab>", function()
	vim.fn.rpcrequest(ensure_job(), "cursortab_apply")
end, { noremap = true, silent = true })
