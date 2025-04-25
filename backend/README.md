# Karaoke Generator

Công cụ này giúp tạo file karaoke từ một file nhạc MP3 với các chức năng:
- Tách vocal và nhạc nền sử dụng Demucs
- Xử lý audio với FFmpeg 
- Tạo timestamp cho từng từ sử dụng Montreal Forced Aligner (MFA)
- Tạo file JSON chứa thông tin karaoke

## Yêu cầu hệ thống

- Python 3.9 hoặc cao hơn
- Go 1.16 hoặc cao hơn
- FFmpeg
- Conda (Miniconda hoặc Anaconda)
- Ít nhất 16GB RAM và 15GB dung lượng đĩa trống

## Cài đặt

### Bước 1: Cài đặt Conda

#### Đối với MacOS:
```bash
curl -O https://repo.anaconda.com/miniconda/Miniconda3-latest-MacOSX-x86_64.sh
bash Miniconda3-latest-MacOSX-x86_64.sh
```

#### Đối với Linux:
```bash
wget https://repo.anaconda.com/miniconda/Miniconda3-latest-Linux-x86_64.sh
chmod +x Miniconda3-latest-Linux-x86_64.sh
./Miniconda3-latest-Linux-x86_64.sh
```

Khởi tạo Conda:
```bash
# Thêm conda vào PATH (thay đổi đường dẫn nếu cần)
eval "$(/home/user/miniconda3/bin/conda shell.bash hook)"
conda init
```

### Bước 2: Cài đặt Demucs

Demucs là công cụ dùng để tách vocal và nhạc nền:

```bash
conda create -n demucs_env python=3.10
conda activate demucs_env
pip install numpy==1.24.3
pip install demucs
```

### Bước 3: Cài đặt FFmpeg

FFmpeg dùng để xử lý audio:

#### MacOS:
```bash
brew install ffmpeg
```

#### Linux:
```bash
sudo apt-get update && sudo apt-get install -y ffmpeg
```

### Bước 4: Cài đặt Montreal Forced Aligner (MFA)

MFA dùng để căn chỉnh thời gian cho từng từ trong bài hát:

```bash
conda create -n mfa python=3.10
conda activate mfa
conda config --add channels conda-forge
conda install montreal-forced-aligner
```

Kiểm tra cài đặt:
```bash
mfa version
```

### Bước 5: Tải mô hình tiếng Việt cho MFA

```bash
conda activate mfa
mfa models download dictionary vietnamese_mfa
mfa models download acoustic vietnamese_mfa
```

Kiểm tra mô hình đã tải:
```bash
mfa models list acoustic
mfa models list dictionary
```

### Bước 6: Cài đặt Go

#### MacOS:
```bash
brew install go
```

#### Linux:
```bash
sudo apt-get install golang
```

### Bước 7: Biên dịch chương trình

```bash
# Clone repository hoặc tải source code
git clone https://github.com/ducnguyen/karaoke.git
cd karaoke

# Khởi tạo Go module
go mod init github.com/ducnguyen/karaoke

# Biên dịch chương trình
go build
```

## Sử dụng

### Chuẩn bị dữ liệu đầu vào
1. Chuẩn bị file nhạc MP3
2. Chuẩn bị file lời bài hát `.lab` theo định dạng:
   - Mỗi dòng là một câu trong bài hát
   - Không có thời gian, chỉ có lời

### Chạy chương trình

```bash
./karaoke
```

Hoặc chỉ định tham số đầu vào:

```bash
./karaoke -input="path/to/your/song.mp3"
```

### Quy trình xử lý

1. Tách vocal và nhạc nền
```bash
conda activate demucs_env
demucs --two-stems=vocals --out="./output" "your_song.mp3"
```

2. Chuyển đổi âm thanh từ 44.1kHz sang 48kHz
```bash
ffmpeg -i vocals.wav -ar 48000 vocals_48k.wav
```

3. Tạo timestamp với MFA
```bash
conda activate mfa
mfa align input_files vietnamese_mfa vietnamese_mfa output_files
```

## Cấu trúc thư mục
```
karaoke/
├── input/              # Thư mục chứa file đầu vào
│   └── vocals_48k.lab  # File lời bài hát
├── output/             # Thư mục chứa kết quả
│   ├── vocals_48k.ogg  # File vocal
│   ├── no_vocals_48k.ogg  # File nhạc nền
│   └── output.json     # File timestamp cho karaoke
├── karaoke             # File thực thi (sau khi biên dịch)
├── song_generator.go   # Mã nguồn chính
└── textgrid_converter.go  # Công cụ chuyển đổi TextGrid sang JSON
```

## Xử lý sự cố

### Lỗi "mfa: command not found"
- Đảm bảo đã kích hoạt môi trường MFA: `conda activate mfa`
- Kiểm tra cài đặt: `which mfa`

### Lỗi khi biên dịch Go
- Đảm bảo đã khởi tạo Go module: `go mod init github.com/ducnguyen/karaoke`
- Kiểm tra các thư viện phụ thuộc: `go mod tidy`