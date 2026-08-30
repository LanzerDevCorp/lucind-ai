package packetauthor

import (
	"sort"
	"strconv"
)

func diagnostic(rank int, field, code, message string) Diagnostic {
	return Diagnostic{PacketIndex: -1, Rank: rank, Field: field, ItemIndex: -1, Code: code, Message: message}
}
func orderDiagnostics(in Diagnostics) Diagnostics {
	sort.SliceStable(in, func(i, j int) bool {
		a, b := in[i], in[j]
		if a.PacketIndex != b.PacketIndex {
			return a.PacketIndex < b.PacketIndex
		}
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		if a.ItemIndex != b.ItemIndex {
			return a.ItemIndex < b.ItemIndex
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		return a.Message < b.Message
	})
	out := in[:0]
	last := ""
	for _, d := range in {
		key := d.Key() + ":" + strconv.Quote(d.Message)
		if len(out) == 0 || key != last {
			out = append(out, d)
			last = key
		}
	}
	return out
}

func diagnosticsError(d Diagnostics) error {
	if len(d) == 0 {
		return nil
	}
	return orderDiagnostics(d)
}
