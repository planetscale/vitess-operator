#!/bin/bash

ROOT=$(pwd)

if [ "$OLD_VITESS_VERSION" == "" ]; then
  echo "Set the env var OLD_VITESS_VERSION with the previous version of Vitess. This value will be used to prepare the upgrade endtoend tests."
  exit 1
fi

if [ "$NEW_VITESS_VERSION" == "" ]; then
  echo "Set the env var NEW_VITESS_VERSION with the newest version of Vitess"
  exit 1
fi

if [ "$NEW_OPERATOR_VERSION" == "" ]; then
  echo "Set the env var NEW_OPERATOR_VERSION with the new version of the operator."
  exit 1
fi

if [ "$NEXT_OPERATOR_VERSION" == "" ]; then
  echo "Set the env var NEXT_OPERATOR_VERSION with the next dev version version of the operator."
  exit 1
fi


function updateVitessImages() {
  old_vitess_version=$1
  new_vitess_version=$2

  operator_files=$(find -E $ROOT/test/endtoend/operator/* -name "*.yaml" | grep -v "101_initial_cluster.yaml")
  sed -i.bak -E "s/vitess\/lite:([^-]*)(-rc[0-9]*)?(-mysql..)?/vitess\/lite:v$new_vitess_version\3/g" $operator_files
  sed -i.bak -E "s/vitess\/vtadmin:([^-]*)(-rc[0-9]*)?(-mysql..)?/vitess\/vtadmin:v$new_vitess_version\3/g" $operator_files
  sed -i.bak -E "s/vitess\/lite:([^-]*)(-rc[0-9]*)?(-mysql..)?/vitess\/lite:v$new_vitess_version\3\"/g" $ROOT/pkg/apis/planetscale/v2/defaults.go
  sed -i.bak -E "s/vitess\/lite:([^-]*)(-rc[0-9]*)?(-mysql..)?/vitess\/lite:v$old_vitess_version\3/g" $ROOT/test/endtoend/operator/101_initial_cluster.yaml

  rm -f $(find -E $ROOT/test/endtoend/operator/ -name "*.yaml.bak") $ROOT/pkg/apis/planetscale/v2/defaults.go.bak
}

function updateVersion() {
  version=$1

  sed -i.bak -E "s/Version = \"(.*)\"/Version = \"$version\"/g" $ROOT/version/version.go
  rm -f $ROOT/version/version.go.bak
}


git_status_output=$(git status --porcelain)
if [ "$git_status_output" == "" ]; then
  	echo so much clean
else
    echo "cannot do release with dirty git state"
    exit 1
fi

updateVersion $NEW_OPERATOR_VERSION
updateVitessImages $OLD_VITESS_VERSION $NEW_VITESS_VERSION $NEW_OPERATOR_VERSION

git add --all
git commit -n -s -m "Release commit for $NEW_OPERATOR_VERSION"
git tag -m Version\ $NEW_OPERATOR_VERSION v$NEW_OPERATOR_VERSION

updateVersion $NEXT_OPERATOR_VERSION

git add --all
git commit -n -s -m "Back to dev mode"

echo ""
echo "-----------------------"
echo ""
echo "Please push the new git tag:"
echo ""
echo "   git push origin v$NEW_OPERATOR_VERSION"
echo ""
echo "And push your current branch in order to open a Pull Request against the release branch."
echo ""
echo ""
