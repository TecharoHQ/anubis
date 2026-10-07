# shellcheck shell=bash

# BSD `find` has no `-printf` flag, so you have to get file timestamps from
# `stat`. Beautifully, both `stat` implementations differ in how these
# timestamps are extracted. Figure out which flag you need with a small test
# because people can run BSD coreutils (or busybox?).
if stat -f '%m' . >/dev/null 2>&1; then
	stat_mtime_args=(-f '%m')
else
	stat_mtime_args=(-c '%Y')
fi

# Implements the subset of `find -printf` that's actually needed here.
mtimes() {
	find "$@" -exec stat "${stat_mtime_args[@]}" {} +
}

# all_populated PATTERN DIR... succeeds when every DIR holds at least one file
# matching PATTERN. Timestamps alone can't answer "are the artifacts there?":
# a run that dies partway through leaves one output directory filled and the
# next one empty, and every source file is older than whatever that run managed
# to write. Check that the files exist before believing the clock.
all_populated() {
	local pattern="${1}"
	shift

	local dir
	for dir in "$@"; do
		compgen -G "${dir}/${pattern}" >/dev/null || return 1
	done
}

# Node can crash with SIGILL on riscv64 even with Liftoff disabled.
# Skip it when choosing a runtime for wasm-opt and wasm2js.
node_runs_wasm() {
	command -v node >/dev/null 2>&1 && [ "$(uname -m)" != "riscv64" ]
}
