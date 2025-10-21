echo Building hue-cli...
go build -o hue
if [ $? -eq 0 ]; then
    echo "Build successful! Run ./hue --help to get started."
else
    echo "Build failed!"
    exit 1
fi