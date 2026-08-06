#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2026 AFCMS <afcm.contact@gmail.com>
# SPDX-License-Identifier: LGPL-3.0-or-later

set -euo pipefail

if (( $# != 2 )); then
	echo "usage: bundle-libraries BUILD_DIRECTORY EXECUTABLE" >&2
	exit 2
fi

build_directory=$(readlink -e -- "$1")
executable="$build_directory/$2"
library_directory="$build_directory/lib"

if [[ ! -x "$executable" ]]; then
	echo "executable does not exist or is not executable: $executable" >&2
	exit 1
fi

declare -A resolved_libraries=()
declare -A visited_libraries=()

is_glibc_library() {
	case "$1" in
		ld-linux-x86-64.so.2 | \
		libanl.so.1 | \
		libBrokenLocale.so.1 | \
		libc.so.6 | \
		libdl.so.2 | \
		libm.so.6 | \
		libnss_*.so.2 | \
		libpthread.so.0 | \
		libresolv.so.2 | \
		librt.so.1 | \
		libthread_db.so.1 | \
		libutil.so.1)
			return 0
			;;
	esac

	return 1
}

collect_libraries() {
	local object=$1
	local library
	local library_name
	local resolved_library

	while IFS= read -r library; do
		library_name=$(basename -- "$library")
		if is_glibc_library "$library_name"; then
			continue
		fi

		resolved_library=$(readlink -e -- "$library")
		if [[ -z "${resolved_libraries[$library_name]+present}" ]]; then
			resolved_libraries[$library_name]=$resolved_library
		elif [[ "${resolved_libraries[$library_name]}" != "$resolved_library" ]]; then
			echo "conflicting libraries named $library_name" >&2
			exit 1
		fi

		if [[ -n "${visited_libraries[$resolved_library]+present}" ]]; then
			continue
		fi
		visited_libraries[$resolved_library]=1
		collect_libraries "$resolved_library"
	done < <(
		LC_ALL=C ldd "$object" | awk '
			$2 == "=>" && $3 ~ /^\// { print $3 }
			$1 ~ /^\// { print $1 }
		'
	)
}

collect_libraries "$executable"
install -d -- "$library_directory"

mapfile -t library_names < <(printf '%s\n' "${!resolved_libraries[@]}" | sort)
for library_name in "${library_names[@]}"; do
	destination="$library_directory/$library_name"
	cp --dereference --preserve=mode,timestamps -- \
		"${resolved_libraries[$library_name]}" \
		"$destination"
	patchelf --set-rpath '$ORIGIN' "$destination"
done

patchelf --set-rpath '$ORIGIN/../lib' "$executable"

printf 'Bundled %d shared libraries for %s\n' \
	"${#library_names[@]}" \
	"$executable"
