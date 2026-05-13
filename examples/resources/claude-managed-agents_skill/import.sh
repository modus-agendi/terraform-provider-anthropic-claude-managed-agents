#!/usr/bin/env bash
# Import an existing custom skill by its `skill_*` id.
#
# The Skills API does not return file contents on read, so after import you
# must populate `source_dir` and `display_title` in your HCL before the
# next apply. If your local directory matches the server-side content, the
# computed `content_hash` will pin and no new version will be uploaded.

terraform import claude-managed-agents_skill.report_builder skill_01HqR2k7vXbZ9mNpL3wYcT8f
