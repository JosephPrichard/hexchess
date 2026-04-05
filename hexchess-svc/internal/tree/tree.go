package tree

// ElementsAtDepth returns the number of elements at a given depth.
// Each node holds 2 elements. At depth d, there are 2^(N-d) nodes.
//
// N=3, d=2 → 4 elements
//
//	depth 3 │  [e e]
//	        │  /   \
//	depth 2 │ [e e] [e e]   ← 4 elements (2 nodes × 2)
//	        │ / \   / \
//	depth 1 │[ee][ee][ee][ee]
func ElementsAtDepth(maxDepth, depth int) int { return 1 << (maxDepth - depth + 1) }

// ElementsAtFirstDepth returns the number of elements at the leaf level (depth 1).
// This is the widest row — 2^N elements total.
//
// N=3 → 8 elements
//
//	depth 3 │      [e e]
//	        │      /   \
//	depth 2 │   [e e] [e e]
//	        │   / \   / \
//	depth 1 │ [ee][ee][ee][ee]   ← 8 elements
//	           ^^^^^^^^^^^^^^^^
func ElementsAtFirstDepth(maxDepth int) int { return ElementsAtDepth(maxDepth, 1) }

// NodesAtDepth returns the number of nodes at a given depth.
// Root (depth N) has 1 node; each level down doubles the count.
//
// N=3, d=1 → 4 nodes
//
//	depth 3 │      [ ]          1 node
//	        │     /   \
//	depth 2 │   [ ]   [ ]       2 nodes
//	        │   / \   / \
//	depth 1 │ [ ] [ ] [ ] [ ]   ← 4 nodes
//	           ^^^^^^^^^^^^^^^^^^^
func NodesAtDepth(maxDepth, depth int) int { return 1 << (maxDepth - depth) }

// TotalNodes returns the total node count across all depths.
// Geometric series: 1 + 2 + 4 + ... + 2^(N-1) = 2^N - 1
//
// N=3 → 7 nodes
//
//	depth 3 │      [ ]          1
//	        │     /   \
//	depth 2 │   [ ]   [ ]       2
//	        │   / \   / \
//	depth 1 │ [ ] [ ] [ ] [ ]   4
//	                             ─
//	                             7 total
func TotalNodes(maxDepth int) int { return (1 << maxDepth) - 1 }
