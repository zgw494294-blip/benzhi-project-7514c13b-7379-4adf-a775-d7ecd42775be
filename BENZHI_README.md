# BENZHI_README

## 项目说明
- 项目：benzhi-project-7514c13b-7379-4adf-a775-d7ecd42775be
- 项目用途：完整实现面向地质钻探团队的岩芯编录复核服务，覆盖任务建立、连续孔段编录、异常证据与处置、取样冻结和退回补正、实验检测质量复核、不可变交接凭据签发及校验，并使用带版本、序号和校验和的原子 JSON 账本持久化。
- Go 工具链：`golang:1.23`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/corelog-server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-7514c13b-7379-4adf-a775-d7ecd42775be-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-7514c13b-7379-4adf-a775-d7ecd42775be-arm64 linux/arm64
docker run -it benzhi-project-7514c13b-7379-4adf-a775-d7ecd42775be-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/corelog-server -selfcheck -addr=127.0.0.1:19081`
