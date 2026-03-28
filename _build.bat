@echo off
setlocal enabledelayedexpansion

:: All output goes to both console and build.log via PowerShell Tee-Object.
set "LOGFILE=%~dp0build.log"
if "%~1"=="__INNER__" goto :main
powershell -NoProfile -Command "& { cmd /c '\"%~f0\" __INNER__' 2>&1 | Tee-Object -FilePath '%LOGFILE%' }"
exit /b %errorlevel%

:main

echo ============================================
echo          GhostSpell Full Build
echo ============================================
echo.

cd /d "%~dp0"

:: Kill any running GhostSpell processes before building.
:: ghostspell.exe spawns ghostai.exe and ghostvoice.exe subprocesses —
:: they must be stopped or the compiler can't overwrite the binaries.
for %%p in (ghostspell.exe ghostai.exe ghostvoice.exe ghost.exe) do (
    tasklist /fi "imagename eq %%p" /nh 2>nul | findstr /i "%%p" >nul 2>&1
    if !errorlevel!==0 (
        echo [pre-build] Stopping %%p...
        taskkill /im %%p /f >nul 2>&1
    )
)
timeout /t 1 /nobreak >nul

:: --clean flag: delete build cache, sources, and binaries — full rebuild from scratch.
if "%~1"=="--clean" (
    echo [clean] Deleting build cache, sources, and binaries...
    if exist "build\llama" rmdir /s /q "build\llama"
    if exist "build\llama-src" rmdir /s /q "build\llama-src"
    if exist "build\whisper" rmdir /s /q "build\whisper"
    if exist "build\whisper-src" rmdir /s /q "build\whisper-src"
    del /q ghostspell.exe ghostai.exe ghost.exe ghostvoice.exe ghostvoice-windows-*.exe 2>nul
    echo [clean] Done — rebuilding everything from scratch.
    echo.
)
if "%~2"=="--clean" (
    if exist "build\llama" rmdir /s /q "build\llama"
    if exist "build\llama-src" rmdir /s /q "build\llama-src"
    if exist "build\whisper" rmdir /s /q "build\whisper"
    if exist "build\whisper-src" rmdir /s /q "build\whisper-src"
    del /q ghostspell.exe ghostai.exe ghost.exe ghostvoice.exe ghostvoice-windows-*.exe 2>nul
)

:: Auto-detect MSYS2 MinGW64 toolchain and add to PATH if present.
if exist "C:\msys64\mingw64\bin\gcc.exe" (
    set "PATH=C:\msys64\mingw64\bin;%PATH%"
)

set LLAMA_VERSION=b8545
set BUILD_DIR=%~dp0build
set LLAMA_SRC=%BUILD_DIR%\llama-src
set LLAMA_OUT=%BUILD_DIR%\llama
set GHOSTAI=0

:: ============================================================
:: Step 0 — Check prerequisites
:: ============================================================
echo [0] Checking prerequisites...
echo.

set MISSING=0

where go >nul 2>&1
if %errorlevel% neq 0 (
    echo   ERROR: 'go' not found. Install Go from https://go.dev/dl/
    set MISSING=1
) else (
    for /f "delims=" %%v in ('go version 2^>^&1') do echo   go ......... OK ^(%%v^)
)

where node >nul 2>&1
if %errorlevel% neq 0 (
    echo   ERROR: 'node' not found. Install Node.js from https://nodejs.org/
    set MISSING=1
) else (
    for /f "delims=" %%v in ('node --version 2^>^&1') do echo   node ....... OK ^(%%v^)
)

where npm >nul 2>&1
if %errorlevel% neq 0 (
    echo   ERROR: 'npm' not found. Install Node.js from https://nodejs.org/
    set MISSING=1
) else (
    for /f "delims=" %%v in ('npm --version 2^>^&1') do echo   npm ........ OK ^(%%v^)
)

if %MISSING%==1 (
    echo.
    echo   Install the missing tools above and try again.
    pause
    exit /b 1
)

:: Ghost-AI toolchain (optional — build falls back to API-only mode without these)
set HAS_CMAKE=0
set HAS_GCC=0
set HAS_GENERATOR=0
set GENERATOR_NAME=

where cmake >nul 2>&1
if !errorlevel!==0 set HAS_CMAKE=1

where gcc >nul 2>&1
if !errorlevel!==0 set HAS_GCC=1

where ninja >nul 2>&1
if !errorlevel!==0 (
    set HAS_GENERATOR=1
    set GENERATOR_NAME=Ninja
)
if !HAS_GENERATOR!==0 (
    where mingw32-make >nul 2>&1
    if !errorlevel!==0 (
        set HAS_GENERATOR=1
        set GENERATOR_NAME=MinGW Makefiles
    )
)

if !HAS_CMAKE!==1 if !HAS_GCC!==1 if !HAS_GENERATOR!==1 (
    for /f "delims=" %%v in ('cmake --version 2^>^&1 ^| findstr /n "." ^| findstr "^1:"') do (
        set "cmake_ver=%%v"
        set "cmake_ver=!cmake_ver:~2!"
        echo   cmake ...... OK ^(!cmake_ver!^)
    )
    for /f "delims=" %%v in ('gcc --version 2^>^&1 ^| findstr /n "." ^| findstr "^1:"') do (
        set "gcc_ver=%%v"
        set "gcc_ver=!gcc_ver:~2!"
        echo   gcc ........ OK ^(!gcc_ver!^)
    )
    echo   generator .. OK ^(!GENERATOR_NAME!^)
    set GHOSTAI=1
)

if !GHOSTAI!==0 (
    echo.
    echo   NOTE: Ghost-AI toolchain not found ^(cmake / gcc / ninja^).
    echo         Building WITHOUT local AI — you can still use API providers
    echo         ^(OpenAI, Anthropic, etc.^) via Settings.
    echo.
    echo         To enable local AI, install MSYS2 ^(https://www.msys2.org^) then run:
    echo           pacman -S mingw-w64-x86_64-toolchain mingw-w64-x86_64-cmake mingw-w64-x86_64-ninja
    echo         and re-run this script.
)

echo.

:: CPU count for parallel builds
set NPROC=%NUMBER_OF_PROCESSORS%
if "%NPROC%"=="" set NPROC=4

:: ============================================================
:: GPU detection — runs BEFORE any build steps so both Ghost-AI
:: and Ghost Voice can use the result.
:: ============================================================
set HAS_CUDA=0
set HAS_VULKAN=0
set USE_MSVC=0

:: Check for CUDA + MSVC. CUDA's nvcc requires cl.exe from Visual Studio.
set "VCVARS="
for /f "delims=" %%i in ('where cl 2^>nul') do set "VCVARS=found"
if "!VCVARS!"=="" (
    :: Try to find and load vcvars64.bat from known VS Build Tools locations.
    for %%p in (
        "C:\Program Files\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
        "C:\Program Files\Microsoft Visual Studio\2022\Community\VC\Auxiliary\Build\vcvars64.bat"
        "C:\Program Files\Microsoft Visual Studio\2022\Professional\VC\Auxiliary\Build\vcvars64.bat"
        "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
    ) do (
        if exist %%p (
            echo   Loading MSVC environment from %%p
            call %%p >nul 2>&1
            set "VCVARS=found"
            goto :vcvars_done
        )
    )
)
:vcvars_done

where nvcc >nul 2>&1
if !errorlevel!==0 (
    where cl >nul 2>&1
    if !errorlevel!==0 (
        set HAS_CUDA=1
        set USE_MSVC=1
        echo   CUDA + MSVC detected — enabling NVIDIA GPU acceleration
    ) else (
        echo   CUDA found but MSVC ^(cl.exe^) missing — install Visual Studio Build Tools
    )
)

:: Fallback: check for Vulkan SDK (works with MinGW).
if !HAS_CUDA!==0 (
    if defined VULKAN_SDK (
        set HAS_VULKAN=1
        echo   Vulkan SDK detected — enabling GPU acceleration ^(fallback^)
    ) else (
        echo   No GPU SDK found — building CPU-only
        echo   For best performance: install Visual Studio Build Tools + CUDA Toolkit
    )
)
echo.

:: ============================================================
:: Step 1 — Build Ghost-AI static libraries (if toolchain found)
:: ============================================================
if !GHOSTAI!==0 goto :skip_ghostai

:: Skip if libraries already built AND version matches.
set /a EXISTING_LIBS=0
if exist "%LLAMA_OUT%\lib" (
    for %%f in ("%LLAMA_OUT%\lib\*.a") do set /a EXISTING_LIBS+=1
)
set CACHED_VER=
if exist "%LLAMA_SRC%\.version" set /p CACHED_VER=<"%LLAMA_SRC%\.version"
if !EXISTING_LIBS! geq 3 (
    if "!CACHED_VER!"=="%LLAMA_VERSION%" (
        echo [1] Ghost-AI libraries already built ^(!EXISTING_LIBS! libs, %LLAMA_VERSION%^) — skipping.
        echo     To rebuild: delete the build\llama folder and re-run.
        echo.
        goto :skip_ghostai
    ) else (
        echo [1] llama.cpp version changed ^(!CACHED_VER! -^> %LLAMA_VERSION%^) — rebuilding...
        rmdir /s /q "%LLAMA_OUT%" 2>nul
    )
)

echo [1] Building Ghost-AI ^(llama.cpp %LLAMA_VERSION%^)...
echo.

:: --- Download llama.cpp source ---
set NEED_DOWNLOAD=1
if exist "%LLAMA_SRC%\.version" (
    set /p CACHED_VER=<"%LLAMA_SRC%\.version"
    if "!CACHED_VER!"=="%LLAMA_VERSION%" (
        echo   Using cached llama.cpp source ^(%LLAMA_VERSION%^)
        set NEED_DOWNLOAD=0
    ) else (
        echo   Version changed ^(!CACHED_VER! -^> %LLAMA_VERSION%^), re-downloading...
        rmdir /s /q "%LLAMA_SRC%" 2>nul
    )
)

if !NEED_DOWNLOAD!==1 (
    echo   Downloading llama.cpp %LLAMA_VERSION%...
    if not exist "%BUILD_DIR%" mkdir "%BUILD_DIR%"
    curl -fsSL "https://github.com/ggml-org/llama.cpp/archive/refs/tags/%LLAMA_VERSION%.tar.gz" -o "%BUILD_DIR%\llama.tar.gz"
    if !errorlevel! neq 0 (
        echo   ERROR: Download failed — falling back to API-only build
        set GHOSTAI=0
        goto :skip_ghostai
    )
    cd /d "%BUILD_DIR%"
    tar xzf llama.tar.gz
    if !errorlevel! neq 0 (
        echo   ERROR: Extract failed — falling back to API-only build
        cd /d "%~dp0"
        set GHOSTAI=0
        goto :skip_ghostai
    )
    if exist "llama-src" rmdir /s /q "llama-src"
    rename "llama.cpp-%LLAMA_VERSION%" llama-src
    echo %LLAMA_VERSION%> "%LLAMA_SRC%\.version"
    del llama.tar.gz 2>nul
    cd /d "%~dp0"
    echo   Downloaded OK
)

:: --- Build static libraries with CMake ---
echo   Compiling static libraries ^(this may take a few minutes^)...

set LLAMA_BUILD=%LLAMA_SRC%\build
if not exist "%LLAMA_BUILD%" mkdir "%LLAMA_BUILD%"

set WIN_FLAGS=-D_WIN32_WINNT=0x0A00

cd /d "%LLAMA_BUILD%"

:: CUDA + MinGW: build llama.cpp as a shared DLL with MSVC+CUDA,
:: then CGo links the DLL via import library.
:: Non-CUDA: build static .a with MinGW as before.
if !HAS_CUDA!==1 (
    :: Remove MinGW from PATH so MSVC doesn't pick up MinGW headers.
    set "SAVED_PATH=!PATH!"
    set "PATH=!PATH:C:\msys64\mingw64\bin=!"
    set "PATH=!PATH:C:\msys64\usr\bin=!"
    cmake .. -G "Ninja" ^
        -DCMAKE_BUILD_TYPE=Release ^
        -DCMAKE_C_FLAGS="%WIN_FLAGS%" ^
        -DCMAKE_CXX_FLAGS="%WIN_FLAGS%" ^
        -DGGML_CUDA=ON ^
        -DGGML_VULKAN=OFF ^
        -DGGML_METAL=OFF ^
        -DGGML_OPENMP=ON ^
        -DLLAMA_BUILD_TESTS=OFF ^
        -DLLAMA_BUILD_EXAMPLES=OFF ^
        -DLLAMA_BUILD_SERVER=OFF ^
        -DBUILD_SHARED_LIBS=ON ^
        -DGGML_NATIVE=OFF ^
        -DGGML_AVX=ON ^
        -DGGML_AVX2=ON ^
        -DGGML_AVX512=OFF ^
        -DGGML_FMA=ON ^
        -DGGML_F16C=ON
) else (
    cmake .. -G "!GENERATOR_NAME!" ^
        -DCMAKE_BUILD_TYPE=Release ^
        -DCMAKE_C_COMPILER=gcc ^
        -DCMAKE_CXX_COMPILER=g++ ^
        -DCMAKE_C_FLAGS="%WIN_FLAGS%" ^
        -DCMAKE_CXX_FLAGS="%WIN_FLAGS%" ^
        -DGGML_STATIC=ON ^
        -DGGML_CUDA=OFF ^
        -DGGML_VULKAN=!HAS_VULKAN! ^
        -DGGML_METAL=OFF ^
        -DGGML_OPENMP=ON ^
        -DLLAMA_BUILD_TESTS=OFF ^
        -DLLAMA_BUILD_EXAMPLES=OFF ^
        -DLLAMA_BUILD_SERVER=OFF ^
        -DBUILD_SHARED_LIBS=OFF ^
        -DGGML_NATIVE=OFF ^
        -DGGML_AVX=ON ^
        -DGGML_AVX2=ON ^
        -DGGML_AVX512=OFF ^
        -DGGML_FMA=ON ^
        -DGGML_F16C=ON
)
if !errorlevel! neq 0 (
    echo   ERROR: CMake configure failed — falling back to API-only build
    cd /d "%~dp0"
    set GHOSTAI=0
    goto :skip_ghostai
)

cmake --build . --config Release -j %NPROC%
if !errorlevel! neq 0 (
    echo   ERROR: Compile failed — falling back to API-only build
    cd /d "%~dp0"
    if defined SAVED_PATH set "PATH=!SAVED_PATH!"
    set GHOSTAI=0
    goto :skip_ghostai
)
cd /d "%~dp0"
:: Restore MinGW PATH after CUDA build.
if defined SAVED_PATH set "PATH=!SAVED_PATH!"

:: --- Install headers + libraries ---
if not exist "%LLAMA_OUT%\include" mkdir "%LLAMA_OUT%\include"
if not exist "%LLAMA_OUT%\lib" mkdir "%LLAMA_OUT%\lib"

:: Headers
copy /y "%LLAMA_SRC%\include\llama.h" "%LLAMA_OUT%\include\" >nul 2>&1
if exist "%LLAMA_SRC%\ggml\include" (
    copy /y "%LLAMA_SRC%\ggml\include\*.h" "%LLAMA_OUT%\include\" >nul 2>&1
)
if exist "%LLAMA_SRC%\include\ggml*.h" (
    copy /y "%LLAMA_SRC%\include\ggml*.h" "%LLAMA_OUT%\include\" >nul 2>&1
)

:: Collect libraries from build tree.
:: CUDA (shared DLL) build: collect .dll + .lib, generate MinGW .a import libs.
:: MinGW (static) build: collect .a files directly.
if !HAS_CUDA!==1 (
    :: Collect DLLs and MSVC import libraries.
    if not exist "%LLAMA_OUT%\bin" mkdir "%LLAMA_OUT%\bin"
    for /r "%LLAMA_BUILD%" %%f in (*.dll) do (
        copy /y "%%f" "%LLAMA_OUT%\bin\" >nul 2>&1
    )
    for /r "%LLAMA_BUILD%" %%f in (*.lib) do (
        copy /y "%%f" "%LLAMA_OUT%\lib\" >nul 2>&1
    )
    :: Generate MinGW-compatible import libraries from DLLs using gendef + dlltool.
    echo   Generating MinGW import libraries from DLLs...
    for %%f in ("%LLAMA_OUT%\bin\*.dll") do (
        set "dllname=%%~nf"
        gendef "%%f" >nul 2>&1
        if exist "!dllname!.def" (
            dlltool -d "!dllname!.def" -l "%LLAMA_OUT%\lib\lib!dllname!.a" -D "%%~nxf" >nul 2>&1
            del "!dllname!.def" 2>nul
        )
    )
) else (
    :: Static MinGW build: collect .a files.
    for /r "%LLAMA_BUILD%" %%f in (*.a) do (
        copy /y "%%f" "%LLAMA_OUT%\lib\" >nul 2>&1
    )
    :: Ensure lib* prefix for MinGW linker.
    for %%f in ("%LLAMA_OUT%\lib\*.a") do (
        set "fname=%%~nxf"
        if not "!fname:~0,3!"=="lib" (
            rename "%%f" "lib!fname!"
        )
    )
)

:: Verify we got libraries
set /a LCOUNT=0
for %%f in ("%LLAMA_OUT%\lib\*.a") do set /a LCOUNT+=1
for %%f in ("%LLAMA_OUT%\lib\*.lib") do set /a LCOUNT+=1
if !LCOUNT!==0 (
    echo   WARNING: No libraries found — falling back to API-only build
    set GHOSTAI=0
) else (
    echo   Ghost-AI ready: !LCOUNT! libraries installed
    if !HAS_CUDA!==1 echo   + CUDA GPU acceleration ^(shared DLLs^)
)
echo.

:skip_ghostai

:: ============================================================
:: Step 1.5 — Build Ghost Voice (whisper.cpp) if toolchain found
:: ============================================================
set GHOSTVOICE=0
if !HAS_CMAKE!==1 if !HAS_GCC!==1 if !HAS_GENERATOR!==1 set GHOSTVOICE=1

if !GHOSTVOICE!==0 goto :skip_ghostvoice

set WHISPER_VERSION=v1.8.4
set WHISPER_SRC=%BUILD_DIR%\whisper-src
set WHISPER_OUT=%BUILD_DIR%\whisper

:: Skip if ghostvoice.exe supports daemon mode (has the --daemon flag compiled in).
set SKIP_WHISPER=0
if exist "%~dp0ghostvoice.exe" (
    findstr /m "daemon" "%~dp0ghostvoice.exe" >nul 2>&1
    if !errorlevel!==0 set SKIP_WHISPER=1
)
if !SKIP_WHISPER!==1 (
    echo [1.5] Ghost Voice already built ^(ghostvoice.exe found^) — skipping.
    echo     To rebuild: delete ghostvoice.exe and the build\whisper folder, then re-run.
    echo.
    goto :skip_ghostvoice
)

echo [1.5] Building Ghost Voice ^(whisper.cpp %WHISPER_VERSION%^)...
echo.

:: --- Download whisper.cpp source ---
set WNEED=1
if exist "%WHISPER_SRC%\.version" (
    set /p WCACHED=<"%WHISPER_SRC%\.version"
    if "!WCACHED!"=="%WHISPER_VERSION%" (
        echo   Using cached whisper.cpp source
        set WNEED=0
    )
)

if !WNEED!==1 (
    echo   Downloading whisper.cpp %WHISPER_VERSION%...
    curl -fsSL "https://github.com/ggml-org/whisper.cpp/archive/refs/tags/%WHISPER_VERSION%.tar.gz" -o "%BUILD_DIR%\whisper.tar.gz"
    if !errorlevel! neq 0 (
        echo   WARNING: Download failed — skipping Ghost Voice
        set GHOSTVOICE=0
        goto :skip_ghostvoice
    )
    cd /d "%BUILD_DIR%"
    tar xzf whisper.tar.gz
    for /d %%d in (whisper.cpp-*) do (
        if exist "whisper-src" rmdir /s /q "whisper-src"
        rename "%%d" whisper-src
    )
    echo %WHISPER_VERSION%> "%WHISPER_SRC%\.version"
    del whisper.tar.gz 2>nul
    cd /d "%~dp0"
    echo   Downloaded OK
)

:: --- Build whisper.cpp libraries with CMake ---
:: CUDA: static libs with MSVC+nvcc (ghostvoice.exe linked with cl.exe — no DLL conflicts with llama ggml DLLs).
:: Non-CUDA: static libs with MinGW (unchanged).
set WHISPER_BUILD=%WHISPER_SRC%\build
:: Clean stale build dir to force a fresh build
if exist "%WHISPER_BUILD%" rmdir /s /q "%WHISPER_BUILD%"
mkdir "%WHISPER_BUILD%"

set WIN_FLAGS=-D_WIN32_WINNT=0x0A00
set WHISPER_CUDA=0

if !HAS_CUDA!==1 (
    :: CUDA build uses a separate build dir to avoid stale-cache issues on fallback.
    set WHISPER_BUILD_CUDA=%WHISPER_SRC%\build-cuda
    if exist "!WHISPER_BUILD_CUDA!" rmdir /s /q "!WHISPER_BUILD_CUDA!"
    mkdir "!WHISPER_BUILD_CUDA!"
    cd /d "!WHISPER_BUILD_CUDA!"
    echo   Compiling whisper.cpp with CUDA ^(MSVC + nvcc^)...
    :: Remove MinGW from PATH so MSVC doesn't pick up MinGW headers.
    set "SAVED_PATH_W=!PATH!"
    set "PATH=!PATH:C:\msys64\mingw64\bin=!"
    set "PATH=!PATH:C:\msys64\usr\bin=!"
    cmake .. -G "Ninja" ^
        -DCMAKE_BUILD_TYPE=Release ^
        -DCMAKE_C_FLAGS="%WIN_FLAGS%" ^
        -DCMAKE_CXX_FLAGS="%WIN_FLAGS%" ^
        -DBUILD_SHARED_LIBS=OFF ^
        -DWHISPER_BUILD_TESTS=OFF ^
        -DWHISPER_BUILD_EXAMPLES=ON ^
        -DGGML_STATIC=ON ^
        -DGGML_CUDA=ON ^
        -DGGML_VULKAN=OFF ^
        -DGGML_METAL=OFF ^
        -DGGML_OPENMP=ON ^
        -DGGML_NATIVE=OFF ^
        -DGGML_AVX=ON ^
        -DGGML_AVX2=ON ^
        -DGGML_AVX512=OFF ^
        -DGGML_FMA=ON ^
        -DGGML_F16C=ON
    if !errorlevel!==0 (
        cmake --build . --config Release -j %NPROC%
        if !errorlevel!==0 (
            set WHISPER_CUDA=1
            :: Point WHISPER_BUILD to the CUDA output for the link step.
            set WHISPER_BUILD=!WHISPER_BUILD_CUDA!
        ) else (
            echo   WARNING: CUDA build failed — falling back to CPU
        )
    ) else (
        echo   WARNING: CUDA CMake failed — falling back to CPU
    )
    :: Restore MinGW PATH.
    if defined SAVED_PATH_W set "PATH=!SAVED_PATH_W!"
)

:: Non-CUDA fallback: MinGW static build (separate build dir, no stale cache).
if !WHISPER_CUDA!==0 (
    echo   Compiling whisper.cpp static libraries ^(CPU^)...
    if exist "%WHISPER_BUILD%" rmdir /s /q "%WHISPER_BUILD%"
    mkdir "%WHISPER_BUILD%"
    cd /d "%WHISPER_BUILD%"
    cmake .. -G "!GENERATOR_NAME!" ^
        -DCMAKE_BUILD_TYPE=Release ^
        -DCMAKE_C_COMPILER=gcc ^
        -DCMAKE_CXX_COMPILER=g++ ^
        -DCMAKE_C_FLAGS="%WIN_FLAGS%" ^
        -DCMAKE_CXX_FLAGS="%WIN_FLAGS%" ^
        -DBUILD_SHARED_LIBS=OFF ^
        -DWHISPER_BUILD_TESTS=OFF ^
        -DWHISPER_BUILD_EXAMPLES=ON ^
        -DGGML_STATIC=ON ^
        -DGGML_CUDA=OFF ^
        -DGGML_VULKAN=!HAS_VULKAN! ^
        -DGGML_METAL=OFF ^
        -DGGML_OPENMP=ON ^
        -DGGML_NATIVE=OFF ^
        -DGGML_AVX=ON ^
        -DGGML_AVX2=ON ^
        -DGGML_AVX512=OFF ^
        -DGGML_FMA=ON ^
        -DGGML_F16C=OFF
    if !errorlevel! neq 0 (
        echo   WARNING: CMake failed — skipping Ghost Voice
        cd /d "%~dp0"
        set GHOSTVOICE=0
        goto :skip_ghostvoice
    )
    cmake --build . --config Release -j %NPROC%
    if !errorlevel! neq 0 (
        echo   WARNING: Build failed — skipping Ghost Voice
        cd /d "%~dp0"
        set GHOSTVOICE=0
        goto :skip_ghostvoice
    )
)
cd /d "%~dp0"

:: --- Install headers + libraries (same pattern as Ghost-AI) ---
:: Wipe output dir to prevent stale headers/libs from previous failed builds
if exist "%WHISPER_OUT%" rmdir /s /q "%WHISPER_OUT%"
mkdir "%WHISPER_OUT%\include"
mkdir "%WHISPER_OUT%\lib"

:: Headers — copy all .h from whisper's include dirs
echo   Collecting headers...
copy /y "%WHISPER_SRC%\include\*.h" "%WHISPER_OUT%\include\" >nul 2>&1
if exist "%WHISPER_SRC%\ggml\include" (
    copy /y "%WHISPER_SRC%\ggml\include\*.h" "%WHISPER_OUT%\include\" >nul 2>&1
)

:: Build ghostvoice.exe — GhostSpell's own speech-to-text helper (pure C++, links whisper static libs).
if !WHISPER_CUDA!==1 (
    echo   Building ghostvoice.exe ^(MSVC + CUDA^)...
    :: Remove MinGW from PATH so cl.exe doesn't pick up MinGW headers.
    set "SAVED_PATH_W=!PATH!"
    set "PATH=!PATH:C:\msys64\mingw64\bin=!"
    set "PATH=!PATH:C:\msys64\usr\bin=!"
    cl /O2 /EHsc /std:c++17 /MD /openmp ^
        /I"%WHISPER_SRC%\include" /I"%WHISPER_SRC%\ggml\include" ^
        "%~dp0ghostvoice\main.cpp" ^
        /Fe:"%~dp0ghostvoice.exe" ^
        /link ^
        /LIBPATH:"!WHISPER_BUILD!\src" ^
        /LIBPATH:"!WHISPER_BUILD!\ggml\src" ^
        /LIBPATH:"%CUDA_PATH%\lib\x64" ^
        whisper.lib ggml.lib ggml-cpu.lib ggml-base.lib ggml-cuda.lib ^
        cudart_static.lib cublas.lib cublasLt.lib ^
        advapi32.lib
    if defined SAVED_PATH_W set "PATH=!SAVED_PATH_W!"
) else if !HAS_VULKAN!==1 (
    echo   Building ghostvoice.exe ^(MinGW + Vulkan^)...
    g++ -O2 -o "%~dp0ghostvoice.exe" "%~dp0ghostvoice\main.cpp" ^
        -I"%WHISPER_SRC%\include" -I"%WHISPER_SRC%\ggml\include" ^
        -L"%WHISPER_BUILD%\src" -L"%WHISPER_BUILD%\ggml\src" ^
        -L"%VULKAN_SDK%\Lib" ^
        -l:libwhisper.a -l:ggml.a -l:ggml-vulkan.a -l:ggml-cpu.a -l:ggml-base.a ^
        -lvulkan-1 ^
        -lstdc++ -lm -lpthread -lkernel32
) else (
    echo   Building ghostvoice.exe ^(MinGW, CPU-only^)...
    g++ -O2 -static -o "%~dp0ghostvoice.exe" "%~dp0ghostvoice\main.cpp" ^
        -I"%WHISPER_SRC%\include" -I"%WHISPER_SRC%\ggml\include" ^
        -L"%WHISPER_BUILD%\src" -L"%WHISPER_BUILD%\ggml\src" ^
        -l:libwhisper.a -l:ggml.a -l:ggml-cpu.a -l:ggml-base.a ^
        -lstdc++ -lm -lpthread -lkernel32
)
if !errorlevel! neq 0 (
    echo   WARNING: ghostvoice.exe build failed
    set GHOSTVOICE=0
) else (
    echo   ghostvoice.exe built OK
    if !WHISPER_CUDA!==1 echo   + CUDA GPU acceleration ^(NVIDIA^)
    if !WHISPER_CUDA!==0 if !HAS_VULKAN!==1 echo   + Vulkan GPU acceleration ^(AMD/Intel/NVIDIA^)
    :: Stage for go:embed
    if not exist "%~dp0voicebin" mkdir "%~dp0voicebin"
    copy /y "%~dp0ghostvoice.exe" "%~dp0voicebin\ghostvoice.exe" >nul 2>&1
)

:: Static libraries — collect all .a from build tree
echo   Collecting libraries...
for /r "%WHISPER_BUILD%" %%f in (*.a) do (
    echo     Found: %%~nxf
    copy /y "%%f" "%WHISPER_OUT%\lib\" >nul 2>&1
)

:: Ensure lib* prefix for MinGW linker
for %%f in ("%WHISPER_OUT%\lib\*.a") do (
    set "wfn=%%~nxf"
    if not "!wfn:~0,3!"=="lib" (
        rename "%%f" "lib!wfn!"
    )
)


:: Verify we got libraries
set /a WCOUNT=0
for %%f in ("%WHISPER_OUT%\lib\*.a") do set /a WCOUNT+=1
if !WCOUNT!==0 (
    echo   WARNING: No whisper libraries found — falling back without Ghost Voice
    set GHOSTVOICE=0
) else (
    echo   Ghost Voice ready: !WCOUNT! libraries installed
)
echo.

:skip_ghostvoice

:: ============================================================
:: Step 2 — Build frontend
:: ============================================================
if !GHOSTAI!==1 (
    echo [2] Building frontend...
) else (
    echo [1] Building frontend...
)
echo.

cd /d "%~dp0gui\frontend"
call npm install
if !errorlevel! neq 0 (
    echo ERROR: npm install failed
    cd /d "%~dp0"
    pause
    exit /b 1
)

echo.
call npm run build
if !errorlevel! neq 0 (
    echo ERROR: frontend build failed
    cd /d "%~dp0"
    pause
    exit /b 1
)
cd /d "%~dp0"
echo.

:: ============================================================
:: Step 3 — Build Go binary
:: ============================================================
:: ghostspell.exe links Ghost-AI (llama.cpp) only.
:: Voice (whisper.cpp) runs via whisper-cli.exe subprocess — no CGo needed.

set MAIN_TAGS=production
if !GHOSTVOICE!==1 if exist "%~dp0voicebin\ghostvoice.exe" set MAIN_TAGS=!MAIN_TAGS! ghostvoice

if !GHOSTAI!==1 (
    echo [3] Building ghostspell.exe with Ghost-AI...
) else (
    echo [2] Building ghostspell.exe ^(API-only mode^)...
)
set CGO_ENABLED=1

go build -tags "!MAIN_TAGS!" -o ghostspell.exe .
if !errorlevel! neq 0 (
    if !GHOSTAI!==1 (
        echo.
        echo   Build failed — retrying without Ghost-AI...
        set GHOSTAI=0
        go build -tags "production" -o ghostspell.exe .
    )
)
if !errorlevel! neq 0 (
    echo ERROR: Go build failed
    pause
    exit /b 1
)

:: Build ghostai.exe (CGo — links llama.cpp, separate process).
if !GHOSTAI!==1 (
    echo   Building ghostai.exe ^(LLM server^)...
    go build -tags "production ghostai" -o ghostai.exe ./cmd/ghostai
    if !errorlevel! neq 0 (
        echo   WARNING: ghostai.exe build failed — local AI will not be available
        set GHOSTAI=0
    ) else (
        echo   ghostai.exe built OK
        :: For CUDA shared DLL builds: copy DLLs next to ghostai.exe.
        if exist "%LLAMA_OUT%\bin\*.dll" (
            echo   Copying CUDA DLLs...
            copy /y "%LLAMA_OUT%\bin\*.dll" "%~dp0" >nul 2>&1
        )
    )
)

:: Build ghost CLI (pure Go, no CGo needed).
echo   Building ghost.exe ^(CLI^)...
go build -tags "production" -o ghost.exe ./cmd/ghost
if !errorlevel! neq 0 (
    echo   WARNING: ghost.exe build failed — CLI will not be available
) else (
    echo   ghost.exe built OK
)

echo.
echo ============================================
echo   BUILD COMPLETE: ghostspell.exe + ghost.exe
if !GHOSTAI!==1 echo   + ghostai.exe ^(local LLM server^)
if !GHOSTVOICE!==1 echo   + Ghost Voice ^(local speech-to-text, embedded^)
if !GHOSTAI!==0 if !GHOSTVOICE!==0 echo   Mode: API-only
echo ============================================
echo.

:: Clear logs for a fresh testing session.
set APPDATA_DIR=%APPDATA%\GhostSpell
if exist "%APPDATA_DIR%\ghostspell.log" (
    del /q "%APPDATA_DIR%\ghostspell.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghostspell.log
)
if exist "%APPDATA_DIR%\ghostvoice.log" (
    del /q "%APPDATA_DIR%\ghostvoice.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghostvoice.log
)
if exist "%APPDATA_DIR%\ghostspell_crash.log" (
    del /q "%APPDATA_DIR%\ghostspell_crash.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghostspell_crash.log
)
if exist "%APPDATA_DIR%\ghost-server.log" (
    del /q "%APPDATA_DIR%\ghost-server.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghost-server.log
)
if exist "%APPDATA_DIR%\ghostai.log" (
    del /q "%APPDATA_DIR%\ghostai.log" 2>nul
    echo Cleared %APPDATA_DIR%\ghostai.log
)
echo.
echo Starting GhostSpell...
start "" ghostspell.exe
echo.
echo Build log saved to: %LOGFILE%
goto :eof
