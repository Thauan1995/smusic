/// security.md section 1.4 ("modo invisível") + backend-go.md section 4's
/// `{type: "visibility", mode: "visible" | "invisible" | "friends_only"}`
/// frame.
///
/// ASSUMPTION flagged for the backend specialist: the task's own concrete
/// scope names this dimension "invisible/friends_only/everyone" (a
/// product-facing name), while backend-go.md's wire literal is `"visible"`
/// for the permissive mode, not `"everyone"`. [everyone] here is the
/// domain-level name (clearer at call sites - "everyone can see me" reads
/// better than "I am visible"); `social_proximity_data`'s DTO maps
/// [everyone] <-> the wire literal `"visible"`. Needs confirmation that
/// `"visible"` really is the backend's intended default-permissive value
/// and not itself a placeholder.
enum ProximityVisibilityMode { invisible, friendsOnly, everyone }
