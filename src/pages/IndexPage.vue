<template>
  <q-page class="row items-center justify-evenly">
    <div class="column q-pa-md full-width">
      <!-- Tiêu đề -->
      <h4 class="text-h4 text-center q-mb-md">Online Karaoke Generator</h4>

      <!-- Form input -->
      <q-card class="full-width q-mb-md">
        <q-card-section>
          <div class="text-h6">Upload your song and lyrics</div>
          <div class="text-subtitle2">We'll generate a karaoke version for you</div>
        </q-card-section>

        <q-separator />

        <q-card-section>
          <div class="row q-col-gutter-md">
            <!-- Audio file input -->
            <div class="col-12 col-md-6">
              <q-file
                v-model="audioFile"
                label="Select Audio File"
                outlined
                accept=".mp3,.wav,.ogg,.flac"
                :disable="isProcessing"
                bottom-slots
              >
                <template v-slot:hint>
                  Supported formats: MP3, WAV, OGG, FLAC
                </template>
                <template v-slot:prepend>
                  <q-icon name="music_note" />
                </template>
              </q-file>
            </div>

            <!-- Language selection -->
            <div class="col-12 col-md-6">
              <q-select
                v-model="language"
                :options="languageOptions"
                label="Lyrics Language"
                outlined
                :disable="isProcessing"
                emit-value
                map-options
              >
                <template v-slot:prepend>
                  <q-icon name="language" />
                </template>
              </q-select>
            </div>

            <!-- Lyrics textarea -->
            <div class="col-12">
              <q-input
                v-model="lyrics"
                type="textarea"
                label="Enter Lyrics"
                outlined
                :disable="isProcessing"
                rows="8"
                bottom-slots
              >
                <template v-slot:hint>
                  Enter the lyrics of your song here
                </template>
                <template v-slot:prepend>
                  <q-icon name="description" />
                </template>
              </q-input>
            </div>
          </div>
        </q-card-section>

        <q-separator />

        <q-card-actions align="right">
          <q-btn
            :loading="isProcessing"
            color="primary"
            label="Generate Karaoke"
            @click="generateKaraoke"
            :disable="!audioFile || !lyrics"
          >
            <template v-slot:loading>
              <q-spinner-dots />
            </template>
          </q-btn>
        </q-card-actions>
      </q-card>

      <!-- Processing banner -->
      <q-banner v-if="isProcessing" class="bg-primary text-white q-mb-md">
        <template v-slot:avatar>
          <q-spinner-dots color="white" />
        </template>
        <div class="text-h6">Processing: {{ currentStep }}</div>
        <div class="q-mt-sm">
          {{ progressMessage }} ({{ Math.round(progressValue) }}%)
        </div>
        <div class="q-mt-sm">
          Estimated time left: {{ estimatedTimeLeft }}
        </div>
        <q-linear-progress
          :value="progressValue / 100"
          color="white"
          class="q-mt-sm"
        />
      </q-banner>

      <!-- Results - simplified with just download button -->
      <q-card v-if="processingComplete" class="full-width">
        <q-card-section>
          <div class="text-h6">Your Karaoke is Ready!</div>
          <div class="text-subtitle2">
            Download your karaoke files and enjoy!
          </div>
        </q-card-section>
        
        <!-- Download button section -->
        <q-card-section>
          <q-btn 
            color="primary" 
            icon="download" 
            label="Download Karaoke Files" 
            @click="downloadKaraokeFiles"
            size="lg"
          />
        </q-card-section>
      </q-card>
    </div>
  </q-page>
</template>

<script setup>
import { ref, onUnmounted } from 'vue';
import axios from 'axios';

// API base URL
const API_BASE_URL = 'http://localhost:8080';

// Form data
const audioFile = ref(null);
const language = ref(1); // Mặc định là tiếng Việt (1)
const lyrics = ref('');

// Processing state
const isProcessing = ref(false);
const sessionId = ref('');
const progressValue = ref(0);
const progressMessage = ref('');
const currentStep = ref('');
const estimatedTimeLeft = ref('calculating...');
const processingComplete = ref(false);

// Language options
const languageOptions = ref([
  { label: 'Vietnamese', value: 1 },
  { label: 'English', value: 2 }
  // Các ngôn ngữ khác có thể thêm sau nếu cần
]);

// Lấy danh sách ngôn ngữ từ API - Không còn cần thiết vì bây giờ chúng ta sử dụng giá trị cố định
// const fetchLanguages = async () => {
//   try {
//     const response = await axios.get(`${API_BASE_URL}/api/languages`);
//     if (response.data && response.data.status === 'success') {
//       languageOptions.value = response.data.languages;
//     }
//   } catch (error) {
//     console.error('Error fetching languages:', error);
//   }
// };

// // Gọi API khi component được tạo
// fetchLanguages();

// Polling interval for progress updates
let progressInterval = null;

// Cleanup on component unmount
onUnmounted(() => {
  if (progressInterval) {
    clearInterval(progressInterval);
  }
});

// Function to download karaoke files
const downloadKaraokeFiles = () => {
  if (!sessionId.value) {
    alert('Session ID not found. Cannot download files.');
    return;
  }
  
  // Open the download URL in a new tab/window
  window.open(`${API_BASE_URL}/api/get-generated-data/${sessionId.value}`, '_blank');
};

// Hàm để poll tiến trình từ API
const pollProgress = async () => {
  if (!sessionId.value) return;
  
  try {
    const response = await axios.get(`${API_BASE_URL}/api/progress/${sessionId.value}`);
    if (response.data) {
      // Cập nhật state từ response
      progressValue.value = response.data.percentage || 0;
      progressMessage.value = response.data.message || '';
      currentStep.value = response.data.currentStep || '';
      estimatedTimeLeft.value = response.data.estimatedTimeLeft || 'calculating...';
      
      // Kiểm tra xem quá trình đã hoàn thành chưa
      if (progressValue.value >= 100) {
        processingComplete.value = true;
        isProcessing.value = false;
        
        // Dừng polling khi hoàn thành
        if (progressInterval) {
          clearInterval(progressInterval);
          progressInterval = null;
        }
      }
    }
  } catch (error) {
    console.error('Error polling progress:', error);
  }
};

// Generate karaoke
const generateKaraoke = async () => {
  if (!audioFile.value || !lyrics.value) {
    return;
  }

  try {
    isProcessing.value = true;
    progressValue.value = 0;
    progressMessage.value = 'Starting process...';
    currentStep.value = 'Initializing';
    estimatedTimeLeft.value = 'calculating...';
    processingComplete.value = false;

    const formData = new FormData();
    formData.append('audio', audioFile.value);
    formData.append('lyrics', lyrics.value);
    formData.append('language', language.value);

    const response = await axios.post(
      `${API_BASE_URL}/api/generate-karaoke-from-upload`,
      formData,
      {
        headers: {
          'Content-Type': 'multipart/form-data'
        }
      }
    );

    if (response.data && response.data.status === 'success') {
      // Lưu session ID từ response
      sessionId.value = response.data.session_id;
      
      // Bắt đầu polling để cập nhật tiến trình
      if (progressInterval) {
        clearInterval(progressInterval);
      }
      progressInterval = setInterval(pollProgress, 1000); // Poll mỗi giây
    } else {
      isProcessing.value = false;
      alert('Error: ' + (response.data.message || 'Unknown error'));
    }
  } catch (error) {
    isProcessing.value = false;
    console.error('Error generating karaoke:', error);
    alert('Error: ' + (error.response?.data?.message || error.message || 'Unknown error'));
  }
};
</script>
