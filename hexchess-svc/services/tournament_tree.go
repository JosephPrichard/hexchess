package svc

// elementsAtDepth returns the number of elements at a given depth.
// Each node holds 2 elements. At depth d, there are 2^(N-d) nodes.
//
// N=3, d=2 → 4 elements
//
//	depth 3 │  [e e]
//	        │  /   \
//	depth 2 │ [e e] [e e]   ← 4 elements (2 nodes × 2)
//	        │ / \   / \
//	depth 1 │[ee][ee][ee][ee]
func elementsAtDepth(maxDepth, depth int) int { return 1 << (maxDepth - depth + 1) }

// elementsAtFirstDepth returns the number of elements at the leaf level (depth 1).
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
func elementsAtFirstDepth(maxDepth int) int { return elementsAtDepth(maxDepth, 1) }

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
func nodesAtDepth(maxDepth, depth int) int { return 1 << (maxDepth - depth) }
