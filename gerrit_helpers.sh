#!/bin/bash

# ==============================================================================
# Gerrit Helper Functions
#
# A collection of shell functions to streamline the local development workflow
# with Gerrit code review.
#
# To use these functions, source this file in your shell's startup script
# (e.g., ~/.bashrc or ~/.zshrc):
#
#   source /path/to/gerrit_helpers.sh
#
# ==============================================================================


# --- Configuration ---
# Set your Gerrit host URL here. This variable must be set for functions
# that interact with the Gerrit API to work.
# Example: GERRIT_HOST="gerrit.example.com"
GERRIT_HOST=""


# 1. Push for Review
# Pushes the current branch to Gerrit for review.
#
# Usage: gerrit_push_for_review [remote] [target_branch]
#   - remote: The name of the remote to push to (default: origin)
#   - target_branch: The name of the target branch (default: master)
#
# Example:
#   gerrit_push_for_review
#   gerrit_push_for_review my_remote main
#
gerrit_push_for_review() {
    local remote="${1:-origin}"
    local target_branch="${2:-master}"

    echo "Pushing for review to '$remote' on branch '$target_branch'..."
    git push "$remote" "HEAD:refs/for/$target_branch"
}


# 2. Amend and Update
# Stages all tracked file changes, amends them to the last commit,
# and pushes the new patch set for review.
#
# Usage: gerrit_amend_and_update [remote] [target_branch]
#
# Example:
#   gerrit_amend_and_update
#
gerrit_amend_and_update() {
    echo "Staging all changes and amending last commit..."
    git add -u
    git commit --amend --no-edit

    gerrit_push_for_review "$1" "$2"
}


# 3. Checkout Change
# Fetches and checks out a specific Gerrit change by its number.
#
# Usage: gerrit_checkout_change <change_number> [patch_set]
#   - change_number: The number of the Gerrit change.
#   - patch_set: The patch set number to check out (default: latest).
#
# Example:
#   gerrit_checkout_change 12345
#   gerrit_checkout_change 12345 3
#
gerrit_checkout_change() {
    if [ -z "$1" ]; then
        echo "Error: Change number is required."
        return 1
    fi

    if [ -z "$GERRIT_HOST" ]; then
        echo "Error: GERRIT_HOST is not configured. Please set it at the top of the gerrit_helpers.sh script."
        return 1
    fi

    local change_number="$1"
    local remote="${3:-origin}"
    local change_json=$(curl -s "https://$GERRIT_HOST/changes/$change_number/detail" | sed '1d')
    local current_revision_sha=$(echo "$change_json" | jq -r '.current_revision')
    local patch_set="${2:-$(echo "$change_json" | jq -r --arg rev "$current_revision_sha" '.revisions[$rev]._number')}"
    local last_two_digits=$(echo "$change_number" | tail -c 3)

    echo "Fetching change $change_number, patch set $patch_set from remote '$remote'..."
    git fetch "$remote" "refs/changes/$last_two_digits/$change_number/$patch_set" && git checkout FETCH_HEAD
}


# 4. Rebase on Master
# Fetches the latest from the remote and rebases the current branch
# on top of the target branch.
#
# Usage: gerrit_rebase_on_master [remote] [target_branch]
#
# Example:
#   gerrit_rebase_on_master
#
gerrit_rebase_on_master() {
    local remote="${1:-origin}"
    local target_branch="${2:-master}"

    echo "Fetching latest changes from '$remote'..."
    git fetch "$remote"

    echo "Rebasing current branch onto '$remote/$target_branch'..."
    git rebase "$remote/$target_branch"
}


# 5. Show Change Status
# Fetches and displays the status of a Gerrit change in the terminal.
# Requires `jq` to be installed (https://stedolan.github.io/jq/).
#
# Usage: gerrit_show_change_status <change_number>
#
# Example:
#   gerrit_show_change_status 12345
#
gerrit_show_change_status() {
    if ! command -v jq &> /dev/null; then
        echo "Error: 'jq' is not installed. Please install it to use this function."
        return 1
    fi

    if [ -z "$1" ]; then
        echo "Error: Change number is required."
        return 1
    fi

    if [ -z "$GERRIT_HOST" ]; then
        echo "Error: GERRIT_HOST is not configured. Please set it at the top of the gerrit_helpers.sh script."
        return 1
    fi

    local change_number="$1"
    local url="https://$GERRIT_HOST/changes/$change_number/detail"

    echo "Fetching status for change $change_number..."

    curl -s "$url" | sed '1d' | jq -r '
        "Subject: \(.subject)\n" +
        "Status: \(.status)\n" +
        "Owner: \(.owner.name) <\(.owner.email)>\n" +
        "Branch: \(.branch)\n" +
        "Updated: \(.updated)\n" +
        "-----\n" +
        "Reviewers:\n" +
        (.reviewers.REVIEWER | map("  - \(.name) <\(.email)>") | join("\n")) +
        "\n-----\n" +
        "Labels:\n" +
        ( .labels | to_entries | map("  \(.key): \(.value.all | map(if .value > 0 then "+\(.value)" else "\(.value)" end + " (\(.name))") | join(", ")) | join("\n") )
    '
}
