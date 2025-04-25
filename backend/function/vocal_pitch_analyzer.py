import json
import librosa
import numpy as np
import math
import os
import time
import re
import argparse

# Convert NumPy types to native Python types for JSON serialization
def convert_to_serializable(obj):
    if isinstance(obj, np.integer):
        return int(obj)
    elif isinstance(obj, np.floating):
        return float(obj)
    elif isinstance(obj, np.ndarray):
        return obj.tolist()
    elif isinstance(obj, dict):
        return {k: convert_to_serializable(v) for k, v in obj.items()}
    elif isinstance(obj, list):
        return [convert_to_serializable(item) for item in obj]
    else:
        return obj

# Hàm chuyển tần số thành số nốt MIDI (0-127)
def freq_to_midi_note(freq):
    if freq <= 0:
        return 0
    
    # Công thức: MIDI = 69 + 12 * log2(freq / 440)
    midi_note = 69 + 12 * math.log2(freq / 440.0)
    
    # Giới hạn trong phạm vi 0 đến 127 (chuẩn MIDI)
    midi_note = max(0, min(127, round(midi_note)))
    
    return int(midi_note)

# Chuyển số MIDI sang tên note
def midi_to_note_name(midi):
    if midi < 0:
        return "N/A"
    
    notes = ['C', 'C#', 'D', 'D#', 'E', 'F', 'F#', 'G', 'G#', 'A', 'A#', 'B']
    note_name = notes[midi % 12]
    octave = midi // 12 - 1
    
    return f"{note_name}{octave}"

# Hàm phân tích pitch cho một đoạn âm thanh
def analyze_pitch(audio, sr):
    """Phân tích pitch của một đoạn âm thanh và trả về note trung bình"""
    stats = {'frequencies': [], 'confidences': [], 'stability': 0}
    
    # Kiểm tra độ dài tối thiểu
    if len(audio) < sr // 10:
        stats['reason'] = f"Đoạn âm thanh quá ngắn: {len(audio)} mẫu < {sr // 10} mẫu"
        stats['short_segment'] = True
        # Thay vì trả về -1, ta vẫn cố gắng phân tích
        # Có thể kết quả kém chính xác nhưng sẽ có giá trị

    # Phân tích pitch
    try:
        pitches, magnitudes = librosa.core.piptrack(y=audio, sr=sr, fmin=50, fmax=2000)
        
        # Lấy các frame có magnitude đủ lớn
        valid_frames = []
        for t in range(pitches.shape[1]):
            index = magnitudes[:,t].argmax()
            pitch = pitches[index,t]
            magnitude = magnitudes[index,t]
            
            # Lọc các pitch hợp lệ (tần số > 0 và magnitude đủ lớn)
            if pitch > 0 and magnitude > 0.02:
                valid_frames.append(pitch)
                stats['frequencies'].append(float(pitch))
                stats['confidences'].append(float(magnitude))
        
        # Nếu không có frame nào hợp lệ
        if not valid_frames:
            stats['reason'] = "Không tìm thấy pitch hợp lệ"
            if 'short_segment' not in stats:
                return -1, stats
            else:
                # Với đoạn ngắn, thử giảm ngưỡng magnitude để bắt được pitch
                for t in range(pitches.shape[1]):
                    index = magnitudes[:,t].argmax()
                    pitch = pitches[index,t]
                    magnitude = magnitudes[index,t]
                    if pitch > 0:  # Chỉ yêu cầu pitch > 0, bỏ qua ngưỡng magnitude
                        valid_frames.append(pitch)
                        stats['frequencies'].append(float(pitch))
                        stats['confidences'].append(float(magnitude))
                if not valid_frames:
                    return -1, stats
        
        # Tính note trung bình từ các frame
        avg_freq = np.mean(valid_frames)
        midi_note = freq_to_midi_note(avg_freq)
        
        # Tính độ ổn định của pitch
        if len(valid_frames) > 1:
            std_dev = np.std(valid_frames)
            stats['pitch_std'] = float(std_dev)
            # Độ ổn định = 1 - (std_dev / avg_freq), được chuẩn hóa về khoảng [0, 1]
            stability = max(0, min(1, 1 - (std_dev / avg_freq / 0.1)))
            stats['stability'] = float(stability)
        
        return int(round(midi_note)), stats
    except Exception as e:
        stats['error'] = str(e)
        return -1, stats

# Hàm kiểm tra có phải ký tự đặc biệt (không phải từ)
def is_special_token(word):
    word = word.strip()
    
    # Các ký tự đặc biệt không phân tích
    special_tokens = ["*", ".", ",", "!", "?", "...", "…", "-", ";", ":", "(", ")", "[", "]"]
    
    # Kiểm tra nếu từ chỉ chứa khoảng trắng hoặc là ký tự đặc biệt
    if not word or word in special_tokens:
        return True
    
    # Kiểm tra nếu từ chỉ chứa các ký tự đặc biệt
    if re.match(r'^[\s\*\.\,\!\?\-\;\:\(\)\[\]]+$', word):
        return True
    
    return False

def main():
    # Create argument parser
    parser = argparse.ArgumentParser(description='Analyze vocal pitch in an audio file')
    parser.add_argument('json_file', help='Path to input JSON file with lyrics')
    parser.add_argument('audio_file', help='Path to input audio file')
    parser.add_argument('--output', dest='output_file', default='output_with_notes.json', 
                      help='Path to output JSON file with notes')
    parser.add_argument('--log', dest='log_file', default='pitch_analysis_log.json',
                      help='Path to log file')
    parser.add_argument('--quiet', action='store_true', help='Reduce verbosity')
    
    # Parse arguments
    args = parser.parse_args()
    
    verbose = not args.quiet
    
    if verbose:
        print("Bắt đầu phân tích pitch")
    start_time = time.time()
    
    # Đường dẫn tới các file từ tham số
    json_file = args.json_file
    audio_file = args.audio_file
    output_file = args.output_file
    log_file = args.log_file
    
    # Kiểm tra file tồn tại
    if not os.path.exists(json_file):
        print(f"Không tìm thấy file {json_file}")
        return
    
    if not os.path.exists(audio_file):
        print(f"Không tìm thấy file {audio_file}")
        return
    
    # Đọc file JSON
    with open(json_file, 'r', encoding='utf-8') as f:
        lyrics_data = json.load(f)
    
    # Đọc file âm thanh
    if verbose:
        print(f"Đang tải file âm thanh {audio_file}...")
    audio, sr = librosa.load(audio_file, sr=None, mono=True)
    if verbose:
        print(f"Đã tải xong file âm thanh: {len(audio)/sr:.2f} giây, Sample rate: {sr} Hz")
    
    # Tổng số từ cần phân tích
    total_words = sum(len(segment["words"]) for segment in lyrics_data["segments"])
    processed_words = 0
    
    # Dictionary lưu log chi tiết
    detailed_log = {
        "file_info": {
            "audio_file": audio_file,
            "lyrics_file": json_file,
            "duration": len(audio)/sr,
            "sample_rate": sr,
            "total_words": total_words
        },
        "word_analysis": []
    }
    
    # Phân tích từng phân đoạn
    for segment_idx, segment in enumerate(lyrics_data["segments"]):
        if verbose:
            print(f"Đang phân tích phân đoạn {segment_idx+1}/{len(lyrics_data['segments'])} - {segment['text'][:30]}...")
        
        # Xử lý từng từ trong phân đoạn
        for word_idx, word_data in enumerate(segment["words"]):
            # Hiển thị tiến độ
            processed_words += 1
            if verbose and processed_words % 10 == 0:
                elapsed = time.time() - start_time
                estimated_total = elapsed / processed_words * total_words
                remaining = estimated_total - elapsed
                print(f"  Tiến độ: {processed_words}/{total_words} từ - Còn lại: {remaining:.1f}s")
            
            # Lấy thông tin từ
            word = word_data["word"]
            start_time_sec = word_data["start"]
            end_time_sec = word_data["end"]
            
            # Log cho từ hiện tại
            word_log = {
                "word": word,
                "start_time": start_time_sec,
                "end_time": end_time_sec
            }
            
            # Kiểm tra nếu là từ đặc biệt thì gán note = -1 và bỏ qua việc phân tích
            if is_special_token(word):
                word_data["note"] = -1
                word_log["is_special"] = True
                word_log["note"] = -1
                detailed_log["word_analysis"].append(word_log)
                continue
            
            # Chuyển đổi thời gian sang mẫu (samples)
            start_sample = int(start_time_sec * sr)
            end_sample = int(end_time_sec * sr)
            
            # Lấy đoạn âm thanh tương ứng
            if start_sample >= len(audio) or end_sample > len(audio):
                word_data["note"] = -1  # Nếu vượt quá độ dài audio
                word_log["error"] = "out_of_range"
                word_log["note"] = -1
                detailed_log["word_analysis"].append(word_log)
                continue
            
            word_audio = audio[start_sample:end_sample]
            
            # Phân tích pitch
            note, stats = analyze_pitch(word_audio, sr)
            
            # Thêm thông tin note vào dữ liệu từ
            word_data["note"] = note
            
            # In thông tin gỡ lỗi cho các từ có note = -1 không phải là đặc biệt
            if verbose and note == -1 and not is_special_token(word):
                word_duration = end_time_sec - start_time_sec
                print(f"⚠️ Từ '{word}' bị note = -1:")
                print(f"   - Thời gian: {start_time_sec:.2f}s - {end_time_sec:.2f}s (độ dài: {word_duration:.4f}s)")
                print(f"   - Số mẫu: {len(word_audio)} (cần tối thiểu {sr // 10})")
                print(f"   - Lý do: {stats.get('reason', 'không rõ')}")
            
            # Thêm thông tin vào log
            word_log["note"] = note
            word_log["stats"] = stats
            detailed_log["word_analysis"].append(word_log)
    
    # Lưu kết quả ra file mới
    with open(output_file, 'w', encoding='utf-8') as f:
        json.dump(convert_to_serializable(lyrics_data), f, ensure_ascii=False, indent=4)
    
    # Lưu log chi tiết
    with open(log_file, 'w', encoding='utf-8') as f:
        json.dump(convert_to_serializable(detailed_log), f, ensure_ascii=False, indent=4)
    
    total_time = time.time() - start_time
    if verbose:
        print(f"Hoàn thành phân tích! Tổng thời gian: {total_time:.2f} giây")
        print(f"Kết quả đã được lưu vào {output_file}")
        print(f"Log chi tiết đã được lưu vào {log_file}")

if __name__ == "__main__":
    main() 