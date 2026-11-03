<template>
  <!-- 页面由项目信息和摄影配图组成；例如桌面端并排显示，窄屏时按上下顺序显示。 -->
  <main class="foundation-shell">
    <section class="project-panel" aria-labelledby="page-title">
      <header class="brand">
        <span class="brand-symbol" aria-hidden="true">P</span>
        <span>PulseFrame</span>
      </header>

      <div class="project-content">
        <p class="eyebrow">工程基座</p>
        <h1 id="page-title">PulseFrame</h1>
        <p class="intro">
          当前保留网页工程与后端运行基础，后续业务模块将按需求逐步加入。
        </p>

        <!-- 用名称与对应值展示前端、后端和当前状态，便于逐项阅读。 -->
        <dl class="foundation-list">
          <div>
            <dt>Web</dt>
            <dd>Vue 3 + TypeScript</dd>
          </div>
          <div>
            <dt>API</dt>
            <dd>Go + Gin</dd>
          </div>
          <div>
            <dt>状态</dt>
            <dd>基座可用</dd>
          </div>
        </dl>
      </div>

      <footer class="panel-footer">
        <span>© 2026 PulseFrame</span>
        <span>Foundation</span>
      </footer>
    </section>

    <!-- 图片来自 public 目录；加载失败或使用屏幕阅读器时，可通过 alt 文字了解图片内容。 -->
    <aside class="visual-panel" aria-label="创作者户外摄影现场">
      <img class="visual-image" src="/creator-capture.jpg" alt="创作者在山脊上记录户外景色" />
      <div class="visual-caption">
        <span>创作者现场</span>
        <span>01 / PULSEFRAME</span>
      </div>
    </aside>
  </main>
</template>
