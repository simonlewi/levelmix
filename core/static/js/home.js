(function () {
    function initCycling() {
        const words = ['mix', 'podcast', 'live set'];
        const el = document.getElementById('cycling-word');
        if (!el) return;
        el.style.cssText = 'display:inline-block;transition:opacity 0.4s ease,transform 0.4s ease;';
        let i = 0;
        setInterval(() => {
            el.style.opacity = '0';
            el.style.transform = 'translateY(-10px)';
            setTimeout(() => {
                i = (i + 1) % words.length;
                el.textContent = words[i];
                el.style.transform = 'translateY(10px)';
                el.style.opacity = '0';
                requestAnimationFrame(() => requestAnimationFrame(() => {
                    el.style.opacity = '1';
                    el.style.transform = 'translateY(0)';
                }));
            }, 400);
        }, 2800);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initCycling);
    } else {
        initCycling();
    }

    const canvas = document.getElementById('hero-wave');
    if (!canvas) return;

    const ctx = canvas.getContext('2d');

    // Precision Console palette
    const ARCTIC = { r: 74,  g: 138, b: 199 }; // #4A8AC7
    const MID    = { r: 122, g: 176, b: 216 }; // #7AB0D8
    const SAND   = { r: 212, g: 169, b: 94  }; // #D4A95E

    function lerpColor(a, b, t) {
        return {
            r: Math.round(a.r + (b.r - a.r) * t),
            g: Math.round(a.g + (b.g - a.g) * t),
            b: Math.round(a.b + (b.b - a.b) * t),
        };
    }

    function paletteColor(bright) {
        return bright < 0.5
            ? lerpColor(ARCTIC, MID,  bright * 2)
            : lerpColor(MID,   SAND,  (bright - 0.5) * 2);
    }

    let dpr = window.devicePixelRatio || 1;

    function resize() {
        const parent = canvas.parentElement;
        canvas.width  = parent.offsetWidth  * dpr;
        canvas.height = parent.offsetHeight * dpr;
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    }
    resize();
    window.addEventListener('resize', resize);

    // canvas.addEventListener('click', function (e) { ... });   // click ripples — off

    // let mouseX, mouseY, mouseActive = false;                  // cursor influence — off
    // canvas.addEventListener('mousemove', ...);
    // canvas.addEventListener('mouseleave', ...);

    // Scatter disturbances: each displaces nearby dots off their waveform lines
    // in screen space, using a per-dot deterministic direction so there's no flicker.
    const disturbances = [];
    let t = 0;
    let nextDisturbT = 1.2;

    function draw() {
        const rect = canvas.getBoundingClientRect();
        const W = rect.width;
        const H = rect.height;

        const speed      = 0.00675; // 25% slower
        const amplitude  = 55;
        const cols       = 80;
        const rows       = Math.round(cols * 0.65);
        const tiltAmount = 0.42;
        const depthScale = 0.9;
        const immersion  = 0.08;

        ctx.fillStyle = '#0F0F0D';
        ctx.fillRect(0, 0, W, H);

        // Ambient glow
        const glow = ctx.createRadialGradient(W * 0.38, H * 0.58, 0, W * 0.38, H * 0.58, W * 0.55);
        glow.addColorStop(0,   'rgba(74,138,199,0.06)');
        glow.addColorStop(0.5, 'rgba(122,176,216,0.03)');
        glow.addColorStop(1,   'rgba(212,169,94,0.00)');
        ctx.fillStyle = glow;
        ctx.fillRect(0, 0, W, H);

        t += speed;

        // Auto-spawn a scatter disturbance at a random spot on the wave surface
        if (t >= nextDisturbT) {
            disturbances.push({
                x:     0.1 + Math.random() * 0.8,
                y:     0.1 + Math.random() * 0.65,
                birth: t,
                life:  2.5 + Math.random() * 2.0,
                seed:  Math.random() * 100
            });
            nextDisturbT = t + 1.5 + Math.random() * 2.5;
        }

        for (let i = disturbances.length - 1; i >= 0; i--) {
            if (t - disturbances[i].birth > disturbances[i].life) disturbances.splice(i, 1);
        }

        const diagAngle = -0.35;
        const cosA   = Math.cos(diagAngle);
        const sinA   = Math.sin(diagAngle);
        const cyBase = 0.52 + immersion * 0.6;

        const points = [];

        for (let iz = 0; iz < rows; iz++) {
            for (let ix = 0; ix < cols; ix++) {
                const nx = ix / (cols - 1);
                const nz = iz / (rows - 1);

                const gx = (nx - 0.5) * W * 1.8;
                const gz = (nz - 0.5) * H * 2.2;

                const rx  = gx * cosA - gz * sinA;
                const rz  = gx * sinA + gz * cosA;
                const nrx = rx / W + 0.5;
                const nrz = rz / H + 0.5;

                const wave1 = Math.sin(nrx * 6.5  + t * 2.2)            * amplitude * 0.55;
                const wave2 = Math.sin(nrz * 7.5  + t * 1.5)            * amplitude * 0.40;
                const wave3 = Math.sin((nrx + nrz) * 4 - t * 2.8)       * amplitude * 0.45;
                const wave4 = Math.cos(nrx * 3.5  - nrz * 5 + t * 1.0)  * amplitude * 0.30;
                const ridgeCenter = 0.35 + Math.sin(nrx * 3 + t) * 0.15;
                const ridge = Math.exp(-Math.pow((nrz - ridgeCenter) * 3.5, 2)) * amplitude * 0.8;

                const y3d = wave1 + wave2 + wave3 + wave4 + ridge;

                const tiltOffset    = (0.5 - nx) * tiltAmount * H;   // right side rises
                const perspScale    = depthScale / (depthScale + nz * 0.5);
                const immPerspBoost = 1 + immersion * nz * 1.5;

                let sx = W * 0.55 + gx * perspScale;                 // shifted slightly right
                let sy = H * cyBase + (y3d + tiltOffset - gz * 0.25) * perspScale * immPerspBoost;

                if (sx < -40 || sx > W + 40 || sy < -40 || sy > H + 40) continue;

                // Screen-space scatter: each nearby disturbance displaces this dot
                // off its waveform line. Direction is deterministic per dot+disturbance
                // so the dot moves to a fixed offset (no per-frame flicker).
                for (const d of disturbances) {
                    const age   = t - d.birth;
                    const norm  = age / d.life;
                    // Smooth envelope: quick fade in, long hold, slow drift back
                    const env   = norm < 0.15
                        ? norm / 0.15
                        : norm > 0.70
                            ? 1 - Math.pow((norm - 0.70) / 0.30, 2)
                            : 1.0;
                    const ddx   = nx - d.x;
                    const ddz   = nz - d.y;
                    const dist2 = ddx * ddx + ddz * ddz;
                    const reach = 0.28;
                    if (dist2 >= reach * reach) continue;
                    const falloff = 1 - Math.sqrt(dist2) / reach;
                    // Unique angle per (dot, disturbance) — same every frame, no flicker
                    const angle = Math.sin(nx * 53.1 + nz * 37.9 + d.seed) * Math.PI * 2;
                    const mag   = falloff * env * 8;
                    sx += Math.cos(angle) * mag;
                    sy += Math.sin(angle) * mag;
                }

                const depthFade    = 1 - nz;
                const heightBright = Math.max(0, y3d / (amplitude * 2));
                const ridgeBright  = Math.exp(-Math.pow((nrz - ridgeCenter) * 3, 2));
                const baseBright   = depthFade * depthFade * 0.5 + heightBright * 0.35 + ridgeBright * 0.3;
                const alpha        = Math.max(0.03, Math.min(0.92, baseBright));
                const baseSize     = (0.5 + perspScale) * 0.8;
                const sizeBoost    = 1 + heightBright * 1.5 + ridgeBright * 1.2;
                const size         = Math.max(0.3, Math.min(4.5, baseSize * sizeBoost * (0.4 + depthFade * 0.8)));
                const colorBright  = Math.min(1, heightBright * 1.1 + ridgeBright * 0.5);
                const col          = paletteColor(colorBright);

                points.push({ sx, sy, size, alpha, z: nz, col });
            }
        }

        points.sort((a, b) => b.z - a.z);

        for (const p of points) {
            ctx.globalAlpha = p.alpha;
            ctx.fillStyle   = `rgb(${p.col.r},${p.col.g},${p.col.b})`;
            ctx.beginPath();
            ctx.arc(p.sx, p.sy, p.size, 0, Math.PI * 2);
            ctx.fill();
        }
        ctx.globalAlpha = 1;

        requestAnimationFrame(draw);
    }

    draw();
}());
