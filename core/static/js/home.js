// HTMX event handlers for navigation
document.body.addEventListener('htmx:beforeRequest', function(evt) {
    console.log('Navigation request started');
});

document.body.addEventListener('htmx:afterRequest', function(evt) {
    console.log('Navigation completed');
});

// Animated Waveform Canvas
const canvas = document.getElementById('waveformCanvas');
if (canvas) {
    const ctx = canvas.getContext('2d');

    // Set canvas size
    function resizeCanvas() {
        canvas.width = canvas.offsetWidth;
        canvas.height = canvas.offsetHeight;
    }
    resizeCanvas();
    window.addEventListener('resize', resizeCanvas);

    // Waveform parameters
    const waveCount = 3;
    const waves = [];
    for (let i = 0; i < waveCount; i++) {
        waves.push({
            y: canvas.height / 2,
            length: 0.01 + (i * 0.005),
            amplitude: 30 + (i * 20),
            frequency: 0.01 + (i * 0.005),
            phase: i * Math.PI / 3
        });
    }

    let increment = 0;

    // Gradient colors (legendary orange to teal)
    function drawWave(wave, color, alpha) {
        ctx.beginPath();
        ctx.moveTo(0, canvas.height / 2);

        for (let x = 0; x < canvas.width; x++) {
            const y = canvas.height / 2 +
                Math.sin(x * wave.frequency + increment + wave.phase) * wave.amplitude;
            ctx.lineTo(x, y);
        }

        ctx.strokeStyle = color;
        ctx.globalAlpha = alpha;
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.globalAlpha = 1;
    }

    function animate() {
        ctx.clearRect(0, 0, canvas.width, canvas.height);

        // Draw multiple waves with gradient colors
        drawWave(waves[0], '#e69e39', 0.5); // Orange
        drawWave(waves[1], '#14b8a6', 0.4); // Teal
        drawWave(waves[2], '#3b82f6', 0.3); // Blue

        increment += 0.02;
        requestAnimationFrame(animate);
    }

    animate();
}
