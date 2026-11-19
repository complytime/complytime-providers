# SPDX-License-Identifier: Apache-2.0
# Shared data-format constants; review policy remains in the existing UF files.
def review_commands:
  ["uf.review-council", "uf.review-pr", "uf.address-feedback"];

def nonempty: type == "string" and length > 0 and length <= 4096;
def digest: type == "string" and test("^[0-9a-f]{64}$");
def path_name: IN("plugin", "native");
def command_name: . as $command | review_commands | index($command) != null;
