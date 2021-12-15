@echo off
echo Avviata compilazione protocol buffer
C:\ProtocolBuffer\protoc-3.19.1-win64\bin\protoc.exe --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative proto\grep.proto
echo Compilazione completata
exit