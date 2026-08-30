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
