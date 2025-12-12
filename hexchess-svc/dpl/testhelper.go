package dpl

import "hexchess-svc/infra"

func BeforePgTxnTests(t infra.TestLogger) (*infra.Postgres, func()) {
	return infra.BeforePgTests(t, true, InsertTestData)
}

func BeforeDbTxnTests(t infra.TestLogger) (infra.Databases, func()) {
	return infra.BeforeDbTests(t, true, InsertTestData)
}

func BeforeDbTests(t infra.TestLogger) (infra.Databases, func()) {
	return infra.BeforeDbTests(t, false, InsertTestData)
}
