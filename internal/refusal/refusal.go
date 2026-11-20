// Package refusal is the refusal-reason catalog (ADR 0012): the closed,
// append-only set of codes every refusal names. It is a leaf: it holds no
// enforcement and imports nothing from this module, so the CLI can list the
// catalog offline without linking the control plane (ADR 0004).
//
// A reason, once published, is never removed, renamed, or given a different
// meaning. Adding one also means adding its row to the developer-ergonomics
// spec's catalog table.
package refusal

// Reason is a stable, machine-readable code for why a request was refused.
type Reason string

const (
	NotFound            Reason = "not-found"
	AlreadyExists       Reason = "already-exists"
	InvalidRequest      Reason = "invalid-request"
	OutOfContract       Reason = "out-of-contract"
	BundleRejected      Reason = "bundle-rejected"
	QuotaExceeded       Reason = "quota-exceeded"
	NotOwner            Reason = "not-owner"
	NotPermitted        Reason = "not-permitted"
	SealNotWeakened     Reason = "seal-not-weakened"
	EvidenceNotPostable Reason = "evidence-not-postable"
	UnwitnessedResults  Reason = "unwitnessed-results"
	IntegrationMissing  Reason = "integration-missing"
	EnforcementUnproven Reason = "enforcement-unproven"
	NoSandboxHost       Reason = "no-sandbox-host"
	NotSealed           Reason = "not-sealed"
	WrongState          Reason = "wrong-state"
	PrerequisiteMissing Reason = "prerequisite-missing"
	PromotionBlocked    Reason = "promotion-blocked"
	AuditUnavailable    Reason = "audit-unavailable"
	InternalError       Reason = "internal-error"
)

// Entry is one catalogued reason with its one-line summary.
type Entry struct {
	Code    Reason `json:"code"`
	Summary string `json:"summary"`
}

// Catalog lists every reason in a stable order.
func Catalog() []Entry {
	return []Entry{
		{NotFound, "the named object does not exist; fix the reference (the subject says what kind)"},
		{AlreadyExists, "the name is taken; pick another name or use the existing object"},
		{InvalidRequest, "the request itself is malformed or names an unknown option; fix the request"},
		{OutOfContract, "the declaration goes outside the boundary contract; change the declaration"},
		{BundleRejected, "intake rejected the bundle; fix the manifests the findings name"},
		{QuotaExceeded, "the bundle exceeds the application's sandbox quota; shrink it or choose a capacity mode"},
		{NotOwner, "the sandbox belongs to another identity; use your own sandbox"},
		{NotPermitted, "the actor lacks the role or policy the operation needs; ask someone who holds it"},
		{SealNotWeakened, "the seal is never weakened per sandbox; amend the application's contract instead"},
		{EvidenceNotPostable, "evidence from a user-controlled cluster cannot be posted or promoted; read it locally"},
		{UnwitnessedResults, "test results reported from outside are not accepted; run the suite with cloudbox test"},
		{IntegrationMissing, "the operation needs an integration this control plane does not have; nothing was recorded"},
		{EnforcementUnproven, "the cluster could not be shown to enforce NetworkPolicy; use a CNI that enforces it"},
		{NoSandboxHost, "no registered sandbox host can take the sandbox; register a host first"},
		{NotSealed, "the sandbox's seal is not active; wait for it or re-create the sandbox"},
		{WrongState, "the object exists but is not in a state that allows this; re-read its state"},
		{PrerequisiteMissing, "a declaration or registration the operation depends on is missing; declare or register it"},
		{PromotionBlocked, "the evidence does not support promotion; produce valid evidence"},
		{AuditUnavailable, "the audit record cannot be written, so nothing proceeded; retry later"},
		{InternalError, "the control plane failed in a way it did not anticipate; report it"},
	}
}
