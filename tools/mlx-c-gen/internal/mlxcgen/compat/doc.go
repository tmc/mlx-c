// Package compat implements the generation-time compatibility gates.
//
// Two invariants are enforced, both at generation time rather than in CI:
//
//   - Parity: every function we share with upstream mlx-c must have the same
//     declaration, so installing our fork does not break an upstream consumer.
//     Symbols only we ship are additive and exempt.
//   - Monotonicity: regeneration must not drop a function that the previously
//     committed API lock recorded.
//
// Both gates fail closed. A divergence or a removal passes only when it is
// declared in a committed waiver file that states the divergence and the
// intended resolution; a waiver that no longer matches what the tree does is
// itself a failure.
package compat
