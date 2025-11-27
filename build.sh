# Generate all Go sources (db clients, stringers, mocks, etc.)
echo "Generating go sources"
(cd hexchess-svc && go generate ./...)

# Generate protos for both frontend and backend using the same proto schema
echo "Generating proto sources"
(mkdir -p hexchess-svc/pb && protoc --go_opt=paths=source_relative --go_out=./hexchess-svc/pb --proto_path ./hexchess-pb ./hexchess-pb/messages.proto)
(cd hexchess-ui && mkdir -p src/lib/pb && npx protoc --ts_out src/lib/pb --proto_path ../hexchess-pb ../hexchess-pb/messages.proto)

# Compile a new WASM chess utility library and copy it to a place the frontend can use it
echo "Generating and installing WASM chesslib"
(cd hexchess-svc/cmd/wasm && GOOS=js GOARCH=wasm go build -o chess.wasm)
(cp hexchess-svc/cmd/wasm/chess.wasm hexchess-ui/static/wasm/chess.wasm)