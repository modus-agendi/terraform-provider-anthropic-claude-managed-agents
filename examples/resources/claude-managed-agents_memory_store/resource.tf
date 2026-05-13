# Memory store with safe defaults: terraform destroy archives the store
# (preserves the audit trail) rather than deleting it.
resource "claude-managed-agents_memory_store" "user_prefs" {
  name        = "User Preferences"
  description = "Per-user preferences and project context."
}

# Memory store that opts into hard-delete on destroy. Use this only for
# transient stores where you do not need the memory-version audit trail.
resource "claude-managed-agents_memory_store" "scratch" {
  name              = "scratch"
  description       = "Disposable scratch space."
  delete_on_destroy = true
}

output "user_prefs_id" {
  value = claude-managed-agents_memory_store.user_prefs.id
}
