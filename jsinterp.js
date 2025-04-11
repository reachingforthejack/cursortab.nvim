// this file is a scratchpad at my attempts at deobfuscating some parts of the electron app,
// and is not meant to be a complete or accurate representation of the code
// this isn't run / used for anything.

// mark a in workbench.desktop.main.js will take to orig impl
// mark c in workspace.desktop.main.js is for streaming the next cursor prediction

// what the fuck is this?
const whatTheFuckIsThis = "m4CoTMbqtR9vV1zd";

const context = { generateUuid: r, startOfCpp: l.startOfCpp };

// my attempt at looking through the minified cursor bundle to figure out how everything
// actually works
async function* streamCpp(
	signalContainingStreamCppRequest,
	cppHandle,
	_s, // this seems unused everywhere, so i think we're fine
) {
	let shouldReplaceRange = false;
	let done = false;
	let hasModelInfo = false;
	let segmentFucker = false;

	for (; ;) {
		if (signalContainingStreamCppRequest.signal.aborted) return;

		const flushResult = await cppHandle.flushCpp(s);
		if (flushResult.type === "failure") {
			throw new Error(flushResult.reason);
		}

		// Check if model info exists and hasn't been yielded yet
		if (!hasModelInfo && l.modelInfo !== void 0) {
			hasModelInfo = true;
			yield {
				case: "modelInfo",
				modelInfo: flushResult.modelInfo
			};
		}

		// Check if range to replace exists and hasn't been yielded yet
		if (!shouldReplaceRange && flushResult.rangeToReplaceOneIndexed !== void 0) {
			shouldReplaceRange = true;
			yield {
				case: "rangeToReplaceOneIndexed",
				range: flushResult.rangeToReplaceOneIndexed
			};
		}

		const buffer = flushResult.buffer;
		for (const segment of buffer) {
			if (segment === whatTheFuckIsThis) {
				segmentFucker = true;
				break;
			}
			yield { case: "text", text: segment };
		}

		if (
			!done && flushResult.doneEdit
		) {
			done = true;
			yield { case: "doneEdit" };
			if (flushResult.cursorPredictionTarget !== void 0) {
				yield {
					case: "fusedCursorPrediction",
					prediction: flushResult.cursorPredictionTarget,
				};
			}

			return;
		}

		await new Promise(res => setTimeout(res, 5));
	}
}

async function streamNextCursorPrediction(

) {
	const f = await this.getPartialCursorPredictionRequest({
		editor: e,
		uri: o.uri,
		modelVersion: o.getVersionId(),
		modelValue: h ? "" : o.getValue(),
		getLinterErrors: s,
	});
	f.currentFile !== void 0 && (f.currentFile.relyOnFilesync = h);

	const cursorPredictionArgs = {
		...f,
		modelName: "main",
		diffHistoryKeys: [],
		contextItems: [],
		parameterHints: this.L.getRelevantParameterHints(e),
		lspContexts: [],
		workspaceId: c,
		fileSyncUpdates: u,
		fileVisibleRanges: this.getOpenVisibleRanges(),
	};
}

/**
 * Retrieves partial context around the current cursor and obtains relevant data
 * for cursor-prediction functionality (like diffs and linter errors).
 */
async function getPartialCursorPredictionRequest({
	editor,
	uri: fileUri,
	modelValue: fileContent,
	getLinterErrors,
	modelVersion,
}) {
	// Get any linter errors for this file.
	const linterErrors = getLinterErrors(fileUri);

	// Access the editor's text model.
	const monacoModel = editor.getModel();
	if (monacoModel === null) {
		throw new Error("Editor model is null");
	}

	// Mark a new 'undo/redo' stack state in the editor (Monaco API).
	monacoModel.pushStackElement();

	/**
	 * Helper function to retain only a window of lines around a given lineNumber.
	 * Lines outside the range [lineNumber - wxe, lineNumber + wxe] are replaced with empty strings.
	 */
	function limitContentToNearbyLines(entireText, lineNumber) {
		// Split entire text into lines.
		const lines = entireText.split("\n");

		// wxe presumably means how many lines of context we want to keep around the cursor.
		const windowSize = wxe;
		let startLineIndex = Math.max(0, lineNumber - windowSize);
		let endLineIndex = Math.min(lines.length, lineNumber + windowSize);

		// If the cursor is near the top, extend downward; if it’s near the bottom, extend upward.
		const linesAboveCursorShort = windowSize - lineNumber;
		const linesBelowCursorShort = lineNumber - (lines.length - windowSize);

		if (linesAboveCursorShort > 0) {
			// If we’re too close to the top, expand the bottom portion.
			endLineIndex = Math.min(lines.length, endLineIndex + linesAboveCursorShort);
		} else if (linesBelowCursorShort > 0) {
			// If we’re too close to the bottom, expand the top portion.
			startLineIndex = Math.max(0, startLineIndex - linesBelowCursorShort);
		}

		// Wipe out everything before startLineIndex.
		for (let i = 0; i < startLineIndex; i++) {
			lines[i] = "";
		}
		// Wipe out everything after endLineIndex.
		for (let i = endLineIndex; i < lines.length; i++) {
			lines[i] = "";
		}

		// Join back together with newlines.
		return lines.join("\n");
	}

	// Get the current cursor position from the editor.
	const cursorPosition = editor.getPosition();
	if (cursorPosition === null) {
		throw new Error("[CURSOR PREDICTION] Cursor position is undefined");
	}

	// If the file content is longer than a certain threshold, limit to wxe*2 lines around cursor.
	const lineCount = fileContent.split("\n").length;
	if (lineCount > wxe * 2) {
		fileContent = limitContentToNearbyLines(fileContent, cursorPosition.lineNumber);
	}

	// Possibly extracts some file info or metadata; the method name is obscure but suggests
	// it might not work for notebook URIs. We feed in the text and the current cursor location.
	const currentFileInfo = this.fastCurrentFileInfoDoesNotWorkForNotebooks(
		fileUri,
		fileContent,
		modelVersion,
		cursorPosition
	);

	// Prepare to gather diff history data
	let diffInfo;

	const startTime = performance.now();

	// Attempt to compile “global diff trajectories” with the local provider.
	// "tx.CompileGlobalDiffTrajectories" is presumably a command enumerated in `tx`.
	const compileDiffResult = await this.M.onlyLocalProvider?.runCommand(
		tx.CompileGlobalDiffTrajectories,
		{}
	);

	if (compileDiffResult === undefined) {
		throw new Error(
			"Compile Diff Trajectories not registered in extension host"
		);
	}

	// Wrap up the result in an object. We might merge in more info later.
	diffInfo = {
		fileDiffHistories: compileDiffResult,
		diffHistory: [],
		blockDiffPatches: [],
		mergedDiffHistories: compileDiffResult,
	};

	const endTime = performance.now();
	// Log the time distribution for debugging or analytics.
	this.J.distribution({
		stat: "cursorpredclient.immediatelyFire.diffHistory",
		value: endTime - startTime,
	});

	return {
		...diffInfo,
		linterErrors,
		currentFile: currentFileInfo,
		enableMoreContext: this.F.applicationUserPersistentStorage.cppExtraContextEnabled,
		cppIntentInfo: { source: "line_change" },
	};
}


/*
		  Ei.wrap(
			new qKe({
			  ...g,
			  modelName: this.getModelName(),
			  diffHistoryKeys: [],
			  contextItems: [],
			  parameterHints: this.Ab.getRelevantParameterHints(t),
			  lspSuggestedItems: w,
			  lspContexts: [],
			  filesyncUpdates: [],
			  workspaceId: m,
			  timeSinceRequestStart:
				performance.now() + performance.timeOrigin - l.startOfCpp,
			  timeAtRequestSend: Date.now(),
			}).toBinary(),
		  ),
		  { generateUuid: r, startOfCpp: l.startOfCpp },

*/


async function getAndShowNextPrediction({
	editor,
	triggerCppCallback,
	getLinterErrors,
	source,
	cppSuggestionRange,
}) {
	if (this.F.workspaceUserPersistentStorage.shouldMockCursorPrediction) {
		// possibly a no-op or test behavior
		this.$(editor);
		return;
	}

	// periodically reload config (for example, to pick up new user settings)
	await this.periodicallyReloadCursorPredictionConfig();
	Qm("[CURSOR PREDICTION] createPrediction: called");

	if (!this.isCursorPredictionEnabled() || this.C === true) {
		Qm("[CURSOR PREDICTION] createPrediction: not enabled or currently clearing prediction");
		return;
	}

	if (this.Y()?.cppConfig?.isFusedCursorPredictionModel) {
		Qm("[CURSOR PREDICTION] createPrediction: skipping because fused cursor prediction model is enabled");
		return;
	}

	try {
		Qm("[CURSOR PREDICTION] createPrediction: clearing prediction");
		await this.clearPrediction({ editor, registerReject: true });
		Qm("[CURSOR PREDICTION] createPrediction: cleared prediction");

		const model = editor.getModel();
		if (!model) {
			Qm("[CURSOR PREDICTION] createPrediction: model is null");
			return;
		}
		const selection = editor.getSelection();
		if (selection === null) {
			Qm("[CURSOR PREDICTION] createPrediction: selection is null");
			return;
		}

		if (this.overlapsInlineDiff(model.uri, selection.startLineNumber) === true) {
			Qm("[CURSOR PREDICTION] createPrediction: overlapsInlineDiff");
			return;
		}

		if (model.getLineCount() < eIo) {
			Qm("[CURSOR PREDICTION] createPrediction: model.getLineCount() < CURSOR_PREDICTION_MIN_FILE_LINES");
			return;
		}

		let modelName = this.F.applicationUserPersistentStorage.cursorPredictionState?.model;
		if (modelName === undefined) {
			modelName = _Eo;
		}

		let workspaceId = this.F.workspaceUserPersistentStorage.uniqueCppWorkspaceId;
		if (workspaceId === undefined) {
			workspaceId =
				Math.random().toString(36).substring(2, 15) +
				Math.random().toString(36).substring(2, 15);
			this.F.setWorkspaceUserPersistentStorage("uniqueCppWorkspaceId", workspaceId);
		}

		if (model.uri.scheme === fe.vscodeNotebookCell) {
			return;
		}

		let fileSyncUpdates = [];
		const relyOnFileSync = await this.shouldRelyOnFileSyncForFile(
			this.G.asRelativePath(model.uri),
			model.getVersionId()
		);

		if (relyOnFileSync) {
			fileSyncUpdates = await this.getFileSyncUpdates(
				this.G.asRelativePath(model.uri),
				model.getVersionId()
			);
		}

		Qm("[CURSOR PREDICTION] createPrediction: getting partial cursor prediction request");
		const partialRequest = await this.getPartialCursorPredictionRequest({
			editor,
			uri: model.uri,
			modelVersion: model.getVersionId(),
			modelValue: relyOnFileSync ? "" : model.getValue(),
			getLinterErrors,
		});

		if (partialRequest.currentFile !== undefined) {
			partialRequest.currentFile.relyOnFilesync = relyOnFileSync;
		}

		const requestData = {
			...partialRequest,
			modelName,
			diffHistoryKeys: [],
			contextItems: [],
			parameterHints: this.L.getRelevantParameterHints(editor),
			lspContexts: [],
			workspaceId,
			fileSyncUpdates,
			fileVisibleRanges: this.getOpenVisibleRanges(),
		};

		if (this.j !== undefined) {
			this.j.abort();
		}
		this.j = new AbortController();

		const aiClient = await this.aiClient();
		const encryptionHeader = await this.ab();

		let predictionText = "";
		let predictedLineNumber;
		let isOutOfRange = false;

		const generationUuid = Kt();

		const predictionStream = aiClient.streamNextCursorPrediction(requestData, {
			signal: this.j.signal,
			headers: { ...ZEo(generationUuid), ...encryptionHeader },
		});

		let predictedUri;
		Qm("[CURSOR PREDICTION] createPrediction: starting to stream");

		for await (const chunk of predictionStream) {
			const { response } = chunk;

			if (response.case === "fileName") {
				predictedUri = this.G.resolveRelativePath(response.value);
				if (predictedUri === undefined) {
					Qm("[CURSOR PREDICTION] predictedUri is undefined");
				}
			}

			if (response.case === "lineNumber") {
				predictedLineNumber = response.value;
				break;
			}

			if (response.case === "isNotInRange") {
				isOutOfRange = true;
				break;
			}
		}

		this.j?.abort();
		this.j = undefined;

		if (isOutOfRange) {
			Qm("[CURSOR PREDICTION] createPrediction: isNoOp");
			return;
		}

		if (predictedLineNumber === undefined) {
			Qm("[CURSOR PREDICTION] predictedLineNumberInRange is undefined.");
			return;
		}

		const predictionOutcome = await this.createPrediction({
			editor,
			model,
			predictedLineNumberInRange: predictedLineNumber,
			predictionText,
			generationUuid,
			source,
			cppSuggestionRange,
			predictedUri,
		});

		if (
			triggerCppCallback !== undefined &&
			predictionOutcome?.lineNumber !== undefined &&
			this.F.nonPersistentStorage.cppState?.suggestion === undefined
		) {
			const endPosition = new Be(predictionOutcome.lineNumber, 1);
			triggerCppCallback(model.uri, editor, au.CursorPrediction, endPosition);
		}
	} catch {
	}
}


async getFileSyncUpdates(relativeWorkspacePath, requestedModelVersion) {
	try {
		const updates = await this.M.onlyLocalProvider?.runCommand(L6.GetFileSyncUpdates, {
			relativeWorkspacePath,
			requestedModelVersion,
		});
		return updates?.map((item) => KV.fromJson(item)) ?? [];
	} catch (error) {
		error("[CURSOR PREDICTION] error getting file sync updates", error);
		return [];
	}
}


async function shouldRelyOnFileSyncForFile(relativeWorkspacePath, modelVersion) {
	try {
		const rely = await this.M.onlyLocalProvider?.runCommand(
			L6.ShouldRelyOnFileSyncForFile,
			{ relativeWorkspacePath, modelVersion }
		);
		Qm("[CURSOR PREDICTION] should rely on file sync for file", relativeWorkspacePath, rely);
		return rely ?? false;
	} catch (error) {
		error("[CURSOR PREDICTION] error checking if should rely on file sync for file", error);
		return false;
	}
}


async function ab() {
	const encryptionHeader = await this.M.onlyLocalProvider?.runCommand(
		L6.GetFileSyncEncryptionHeader,
		null
	);
	if (encryptionHeader === undefined) {
		throw new Error("Could not retrieve file sync encryption header");
	}
	return encryptionHeader;
}

overlapsInlineDiff(uri, lineNumber) {
	const inlineDiffs = this.F.nonPersistentStorage.inlineDiffs;
	if (inlineDiffs === undefined) return false;

	const inProgressGenerations = this.F.nonPersistentStorage.inprogressAIGenerations.map(
		(gen) => gen.generationUUID
	);

	return inlineDiffs.some((diff) => {
		const isInProgress = inProgressGenerations.includes(diff.generationUUID);
		const isLineInRange =
			lineNumber >= diff.currentRange.startLineNumber &&
			lineNumber <= diff.currentRange.endLineNumberExclusive;
		const isSameUri = Cn.isEqual(diff.uri, uri) || uri.fsPath === diff.uri.fsPath;

		return isInProgress && isLineInRange && isSameUri;
	});
}

doesPredictionMatchUniqueLineInRange({
	model,
	range,
	predictionText,
}) {
	const lines = model.getValue().split("\n").slice(
		range.startLineNumber - 1,
		range.endLineNumberExclusive + 2
	);

	let matchIndex = 0;
	let foundOneMatch = false;

	for (matchIndex = 0; matchIndex < lines.length - 2; matchIndex++) {
		const chunk = lines.slice(matchIndex, matchIndex + 3).join("\n");
		if (chunk.startsWith(predictionText)) {
			if (foundOneMatch) return false;
			foundOneMatch = true;
		}
	}
	return foundOneMatch;
}

async function clearPrediction({
	editor,
	onlyRemoveOverlayWidget,
	registerReject,
}) {
	const prediction = this.F.nonPersistentStorage.cursorPrediction;
	if (prediction !== undefined) {
		this.C = true;

		try {
			if (this.n) {
				this.n.dispose();
				this.n = undefined;
			}

			if (onlyRemoveOverlayWidget === true) {
				return;
			}

			const currentModel = editor.getModel();
			if (currentModel !== null && Cn.isEqual(currentModel.uri, prediction.uri)) {
				currentModel.deltaDecorations([prediction.decorationId], []);

				if (this.w !== undefined) {
					currentModel.deltaDecorations([this.w], []);
					this.w = undefined;
				}
				if (this.y !== undefined) {
					currentModel.deltaDecorations([this.y], []);
					this.y = undefined;
				}

				if (registerReject && prediction !== undefined) {
					this.Q.recordRejectCursorPredictionEvent(currentModel, prediction);
				}
			} else {
				const modelReference = await this.O.createModelReference(prediction.uri);
				try {
					const textEditorModel = modelReference.object.textEditorModel;
					textEditorModel.deltaDecorations([prediction.decorationId], []);

					if (registerReject && prediction !== undefined) {
						this.Q.recordRejectCursorPredictionEvent(textEditorModel, prediction);
					}
				} finally {
					modelReference.dispose();
				}
			}

			if (this.q) {
				this.q.dispose();
				this.q = undefined;
			}

			const overlay = this.cb(editor);
			if (overlay !== undefined) {
				overlay.hide();
			}

			this.F.setNonPersistentStorage("cursorPrediction", undefined);
		} catch (error) {
			error("[CURSOR PREDICTION] error clearing prediction", error);
		} finally {
			this.C = false;
		}
	}
}

