// Enterprise 主题交互脚本
(function () {
  'use strict';

  // 导航滚动阴影
  var header = document.getElementById('siteHeader');
  if (header) {
    var onScroll = function () {
      if (window.scrollY > 10) {
        header.classList.add('scrolled');
      } else {
        header.classList.remove('scrolled');
      }
    };
    window.addEventListener('scroll', onScroll, { passive: true });
    onScroll();
  }

  // 移动端菜单切换
  var toggle = document.getElementById('navToggle');
  var nav = document.getElementById('mainNav');
  if (toggle && nav) {
    toggle.addEventListener('click', function () {
      nav.classList.toggle('open');
    });
    // 点击菜单项后关闭
    nav.querySelectorAll('a').forEach(function (link) {
      link.addEventListener('click', function () {
        nav.classList.remove('open');
      });
    });
  }

  // 图集 Lightbox
  var lightbox = document.getElementById('lightbox');
  var lightboxImg = document.getElementById('lightbox-img');
  if (lightbox && lightboxImg) {
    document.querySelectorAll('.gallery-item').forEach(function (a) {
      a.addEventListener('click', function (e) {
        e.preventDefault();
        lightboxImg.src = a.getAttribute('data-src') || a.getAttribute('href');
        lightbox.style.display = 'flex';
      });
    });
    // ESC 关闭
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') lightbox.style.display = 'none';
    });
  }
})();
