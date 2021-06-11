#! /usr/bin/env bash

set -Eeuo pipefail

functions_sourced=0
(return 0 2>/dev/null) && functions_sourced=1

functions_file_previously_sourced=${functions_file_previously_sourced:-0}

if [ "${functions_file_previously_sourced}" = "1" ]; then
    debug "functions already loaded..."
    return 0
fi

log() {
    printf "%b\n" "$*" 1>&2
}

debug() {
    if [ "${DEBUG:-0}" != "0" ]; then
        log "DEBUG: $*"
    fi
}

warn() {
    log "WARN: $*"
}

###~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~##
#
# FUNCTION: BACKTRACE
#
###~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~##

backtrace() {
    local _start_from_=0

    local params=( "$@" )
    if (( "${#params[@]}" >= "1" ))
        then
            _start_from_="$1"
    fi

    local i=0
    local first=false
    while caller $i > /dev/null
    do
        if test -n "$_start_from_" && (( "$i" + 1   >= "$_start_from_" ))
            then
                if test "$first" == false
                    then
                        log "BACKTRACE IS:"
                        first=true
                fi
                caller $i
        fi
        let "i=i+1"
    done
}

exit_handler() {
    local rc=$?
    debug "performing cleanup"
    debug "will exit with ${rc} after cleanup"
    if [ "${SKIP_TEMP_DIR_CREATION}" = "0" ] && [ -d "${my_temp_dir}" ]; then
        debug "🧹 Cleaning up temp files from ${my_temp_dir}..."
        rm -rf "${my_temp_dir}"
    fi
    debug "exiting with code: ${rc}"
    exit "${rc}"
}

error_handler() {
    local rc=$?
    local func="${FUNCNAME[1]}"
    local line="${BASH_LINENO[0]}"
    local src="${BASH_SOURCE[0]}"

    log "  called from: $func(), $src, line $line"

    #
    # GETTING BACKTRACE:
    # ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~ #
    _backtrace=$( backtrace 2 )

    log "\n$_backtrace\n"

    log "❌ Something went wrong, and we can't recover. A bug report might be needed..."

    debug "exiting with code: ${rc}"
    exit "${rc}"
}

trap exit_handler EXIT
trap error_handler ERR

debug "Running $0"

SKIP_TEMP_DIR_CREATION=${SKIP_TEMP_DIR_CREATION:-0}
if [ "${SKIP_TEMP_DIR_CREATION}" = "0" ]; then
    my_temp_dir="$(mktemp -d "${TMPDIR:-/tmp/}$(basename "${0}").XXXXXXXXXXXX")"
    debug "Created temp dir: ${my_temp_dir}"
fi

# if script is not sourced (called directly)
if [ "${functions_sourced}" = "0" ]; then
    log "❌ functions.sh was not sourced. It should only be called using 'source path/to/functions.sh'"
    exit 1
fi

bright_blue="\e[34;1m"
green="\e[32m"
reset="\e[00m"
bold="\e[1m"

test_binary() {
    local binary_name="${1}"
    local variable_name="${2}"
    local try_help_message="${3}"
    local print_success="${4-0}"

    if binary_location=$(command -v "${binary_name}" 2>&1); then
        export "${variable_name}=${binary_name}"
        if [ "$print_success" = "1" ]; then
            log "${green}✔${reset} ${binary_name} located [${binary_location}]"
        fi
    else
        log "Could not find ${binary_name} (${try_help_message})"
        exit 1
    fi
}

ensure_gnu_binary() {
    local binary_name="${1}"
    local variable_name="${2}"
    local try_help_message="${3}"
    local print_success="${4-0}"

    local gbinary_name="g${binary_name}"

    #sed_help="$(LANG=C sed --help 2>&1)"
    #if echo "${sed_help}" | grep -q "GNU\|BusyBox"; then
    if LANG=C ${binary_name} --version 2>&1 | grep -q "GNU\|BusyBox"; then
        binary_location=$(command -v "${binary_name}")
        if [ "$print_success" = "1" ]; then
            log "${green}✔${reset} Located ${binary_name} and verified it is gnu/busybox [${binary_location}]"
        fi
        export "${variable_name}=${binary_location}"
    elif command -v "${gbinary_name}" &>/dev/null; then
        binary_location=$(command -v "${gbinary_name}")
        if [ "$print_success" = "1" ]; then
            log "${green}✔${reset} Located ${gbinary_name} [${binary_location}]"
        fi
        export "${variable_name}=${binary_location}"
    else
        log "❌ Failed to find GNU ${binary_name} as ${binary_name} or ${gbinary_name}. ${try_help_message}." >&2
        exit 1
    fi
}

test_sed() {
    ensure_gnu_binary sed SED "If you are on Mac: brew install gnu-sed" "$@"
}

test_grep() {
    ensure_gnu_binary grep GREP "If you are on Mac: brew install grep" "$@"
}

test_sort() {
    ensure_gnu_binary sort SORT "If you are on Mac: brew install coreutils" "$@"
}

test_awk() {
    ensure_gnu_binary awk AWK "If you are on Mac: brew install gawk" "$@"
}

test_date() {
    ensure_gnu_binary date DATE "If you are on Mac: brew install coreutils" "$@"
}

test_tar() {
    ensure_gnu_binary tar TAR "If you are on Mac: brew install gnu-tar" "$@"
}

test_find() {
    ensure_gnu_binary find FIND "If you are on Mac: brew install findutils --with-default-names" "$@"
}

test_xargs() {
    ensure_gnu_binary xargs XARGS "If you are on Mac: brew install findutils --with-default-names" "$@"
}

test_jq() {
    test_binary jq JQ "If you are on Mac: brew install jq" "$@"
}

test_asdf() {
    test_binary asdf ASDF "If you are on Mac: brew install asdf" "$@"
}

test_yq() {
    test_binary yq YQ "If you are on Mac: brew install yq" "$@"
}

test_kubeseal() {
    test_binary kubeseal KUBESEAL "If you are on Mac: brew install kubeseal" "$@"
}

test_aws() {
    test_binary aws AWS "If you are on Mac: brew install awscli" "$@"
}

test_docker() {
    test_binary docker DOCKER "Install from: https://docs.docker.com/desktop/"
}

test_spin() {
    test_binary spin SPIN "Install instructions: https://spinnaker.io/setup/spin/"
}

test_http() {
    test_binary http HTTP "If you are on Mac: brew install httpie" "$@"
}

test_jsonnet() {
    test_binary jsonnet JSONNET "Try make dependences.dev.install" "$@"
    JSONNET_PATH="${repo_dir}/vendor:${repo_dir}/lib"
    export JSONNET_PATH
}

test_git() {
    test_binary git GIT "If you are on Mac: brew install git && brew link --overwrite git. Then reopen your terminal." "$@"
}

test_terraform() {
    test_binary terraform TERRAFORM "Try: asdf install" "$@"
}

test_sponge() {
    test_binary sponge SPONGE "If you are on Mac: brew install moreutils" "$@"
}

test_bash() {
    if ! LANG=C /usr/bin/env bash --version 2>&1 | grep -q "version 5"; then
        log "❌ Failed to find bash major version 5. If you are on Mac: brew install bash. Then reopen your terminal."
        exit 1
    fi
}

test_bash

ask_confirmation() {
    log ""
    log ""

    local action="${1:-continue}"
    log "Are you sure you want to ${action}?"
    log "   Only 'yes' will be accepted to approve."
    log ""
    printf "%b" "Enter a value: " 1>&2
    read -r
    log ""
    log ""
    if [[ ! $REPLY =~ ^yes$ ]]; then
        log "Aborting...."
        exit 1
    fi
}

repo_dir="$(dirname "${hack_dir}")"
deploy_dir="${repo_dir}/deploy"

config_jsonnet() {
    local config_file="${1}"
    $JSONNET -e "(import '${config_file}').config_"
}

read_config() {
    local config_file="${1}"

    if [ ! -f "${config_file}" ]; then
        log "❌config file not found at: ${config_file}"
        return 1
    fi

    local config_key
    config_key="${2}"

    local config_contents
    config_contents="$(config_jsonnet "${config_file}")"
    debug "config contents:\n************************\n${config_contents}\n************************\n"

    debug "fetching config key: .${config_key}"
    if value=$(echo "${config_contents}" | $JQ ".${config_key}"); then
        if [ -z "${value}" ]; then
            warn "❌ key ${config_key} has an empty value!"
            return 1
        else
            debug "config value (json): ${value}"
            $JQ -r --null-input "${value}"
        fi
    else
        warn "error reading key: ${config_key}; ${value}"
        return 1
    fi
}

capitalize_first() {
    # shellcheck disable=SC2016
    $AWK '{print toupper(substr($0,0,1))tolower(substr($0,2))}'
}

to_lowercase() {
    # shellcheck disable=SC2016
    $AWK '{print tolower($0)}'
}

yq_pretty() {
    $YQ r -P -d'*' -
}

yaml_to_json_array() {
    $YQ r -P -d'*' --collect --tojson -
}

flatten_manifests() {
    local file="${1}"
    # note that the stdin redirect isn't used unless file == /dev/stdin
    # since jsonnet won't read from stdin unless asked to
    $JSONNET \
        --tla-code-file input="${file}" \
        "${hack_dir}/flatten-manifests.jsonnet" < "${file}"
}

yaml_docs_to_yaml_map() {
    yaml_to_json_array | flatten_manifests "/dev/stdin" | yq_pretty
}

url_yaml_upstream() {
    local contents=""
    for url in "$@"
    do
        debug "fetching url: ${url}"
        contents="$(curl -sSL "${url}")\n---\n${contents}"
    done

    printf "%b" "${contents}" | yaml_docs_to_yaml_map
}

git_yaml_upstream() {
    local repo="${1}"
    local yaml_cat_path="${2}"

    local repo_short_with_git=$(basename "${repo}")
    local repo_short="${repo_short_with_git/.git/}"

    (
        cd "${my_temp_dir}"
        git clone -q --depth=1 "${repo}"
        cd "${repo_short}"
        # shellcheck disable=SC2086
        # it is desirable in this case to allow path globbing
        $AWK 'FNR==1 && NR>1 { printf("\n%s\n\n","---") } 1' $yaml_cat_path | yaml_docs_to_yaml_map
    )
}

render_addon_for_diff() {
    local addon_dir="${1}"
    (
        cd "${addon_dir}"
        terraform init > /dev/null
        terraform apply -auto-approve > /dev/null
        terraform output -json objects | yaml_docs_to_yaml_map
    )
}

upstream_image_to_ecr() {
    local upstream="${1}"
    local aws_account_id="${2}"
    local aws_profile="${3}"
    local aws_region="${4:-us-west-2}"

    # this is a bit hacky, but basically, if the root of the upstream image contains a `.` then likely it's a domain
    # and thus this isn't coming from dockerhub
    # for those images, use them as is
    # otherwise to help indicate where images are originally coming from, we'll prefix them
    repo_prefix="docker.io/"
    base_of_upstream=$(echo "$upstream" | $AWK -F "/" '{print $1}')
    log "checking base_of_upstream: ${base_of_upstream} for periods"
    if [[ "${base_of_upstream}" =~ \. ]]; then
        repo_prefix=""
    fi

    AWS_ARGS=(--profile "${aws_profile}" --region "${aws_region}")

    upstream_final="${repo_prefix}${upstream}"

    local ecr_endpoint="${aws_account_id}.dkr.ecr.us-west-2.amazonaws.com"

    ecr_repo_name="$(echo "${upstream_final}" | cut -d: -f1)"

    log "pulling: ${upstream_final}"
    $DOCKER pull "${upstream_final}"

    log "checking if ecr repo exists: ${ecr_repo_name}"
    aws_command=("${AWS}" "${AWS_ARGS[@]}" ecr describe-repositories --registry-id "${aws_account_id}"  --repository-names "${ecr_repo_name}")
    debug "Running AWS Command: ${aws_command[*]}"
    if ! aws_err="$("${aws_command[@]}" 2>&1 >/dev/null)"; then
        if [[ "${aws_err}" =~ "does not exist" ]]; then
            log "repo does not exist. creating: ${ecr_repo_name}"
            $AWS "${AWS_ARGS[@]}" ecr create-repository --repository-name "${ecr_repo_name}"
        else
            error "${aws_err}"
            exit 1
        fi
    fi

    local ecr_tag="${ecr_endpoint}/${upstream_final}"

    $DOCKER tag "${upstream_final}" "${ecr_tag}"
    $AWS "${AWS_ARGS[@]}" ecr get-login-password | docker login --username AWS --password-stdin "${ecr_endpoint}"

    log "pushing: ${ecr_tag}"
    $DOCKER push "${ecr_tag}"
}

# this is actually asking for pascal case, so the article title is wrong
# https://stackoverflow.com/questions/34420091/spinal-case-to-camel-case
kebabcase_to_pascalcase() {
    local value="${1}"
    echo "${value}" | $SED -r 's/(^|-)(\w)/\U\2/g'
}

# https://stackoverflow.com/questions/12487424/uppercase-first-character-in-a-variable-with-bash
kebabcase_to_camelcase() {
    local value="${1}"
    local pascalcase
    pascalcase="$(kebabcase_to_pascalcase "${value}")"
    echo "${pascalcase,}"
}

camelcase_to_kebabcase() {
    local value="${1}"
    echo "${value}" | $SED -r 's/([a-z0-9])([A-Z])/\1-\L\2/g'
}

functions_file_previously_sourced=1
