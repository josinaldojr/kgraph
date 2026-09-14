package graph

import "testing"

func TestNodeCommunity(t *testing.T) {
	tests := []struct {
		name string
		n    *Node
		want int
	}{
		{"unset", &Node{}, -1},
		{"nil properties", &Node{Properties: nil}, -1},
		{"int value", &Node{Properties: map[string]any{PropertyCommunity: 4}}, 4},
		{"float64 value (JSON round-trip)", &Node{Properties: map[string]any{PropertyCommunity: float64(7)}}, 7},
		{"wrong type", &Node{Properties: map[string]any{PropertyCommunity: "oops"}}, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.Community(); got != tt.want {
				t.Errorf("Community() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNodeIsGodNode(t *testing.T) {
	tests := []struct {
		name string
		n    *Node
		want bool
	}{
		{"unset", &Node{}, false},
		{"true", &Node{Properties: map[string]any{PropertyGodNode: true}}, true},
		{"false", &Node{Properties: map[string]any{PropertyGodNode: false}}, false},
		{"wrong type", &Node{Properties: map[string]any{PropertyGodNode: "yes"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.IsGodNode(); got != tt.want {
				t.Errorf("IsGodNode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNodeDegree(t *testing.T) {
	tests := []struct {
		name string
		n    *Node
		want int
	}{
		{"unset", &Node{}, 0},
		{"int value", &Node{Properties: map[string]any{PropertyDegree: 12}}, 12},
		{"float64 value (JSON round-trip)", &Node{Properties: map[string]any{PropertyDegree: float64(3)}}, 3},
		{"wrong type", &Node{Properties: map[string]any{PropertyDegree: "many"}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.Degree(); got != tt.want {
				t.Errorf("Degree() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNodeCommunityLabel(t *testing.T) {
	tests := []struct {
		name string
		n    *Node
		want string
	}{
		{"unset", &Node{}, ""},
		{"set", &Node{Properties: map[string]any{PropertyCommunityLabel: "Auth"}}, "Auth"},
		{"wrong type", &Node{Properties: map[string]any{PropertyCommunityLabel: 5}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.n.CommunityLabel(); got != tt.want {
				t.Errorf("CommunityLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}
