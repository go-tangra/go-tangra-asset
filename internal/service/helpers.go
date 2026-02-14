package service

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrInt32(i int32) *int32 {
	return &i
}

func ptrFloat64(f float64) *float64 {
	return &f
}

func ptrUint32(u uint32) *uint32 {
	return &u
}

func ptrBool(b bool) *bool {
	return &b
}
