// @ts-nocheck -- ported from @hydrooj/ui-default components/navigation/navigation.page.js (AGPL-3.0).
// Domain switching is intentionally removed; the rest of the upstream behavior is preserved.
import $ from 'jquery'

const MOBILE_CUTOFF = 600
const PADDING = 200
const TOLERANCE = 70

type NavigationOptions = {
  onLogout: () => Promise<void>
}

export function initHydroNavigation(root: ParentNode, options: NavigationOptions) {
  const panel = document.getElementById('panel')
  const menuElement = root.querySelector<HTMLElement>('#menu')
  const mainList = menuElement?.querySelector('.nav__list--main')
  if (!panel || !menuElement || !mainList) return () => {}

  const $panel = $(panel)
  const $overlay = $('.slideout-overlay')
  const $hamburger = $('.header__hamburger .hamburger')
  const $trigger = $('<li class="nav__list-item nav_more" data-dropdown-pos="bottom right" data-dropdown-target="#menu-nav-more"><a href="javascript:;" class="nav__item">更多 <span class="icon icon-expand_more"></span></a></li>')
  const $menu = $('<ol class="dropdown-target menu menu_more" id="menu-nav-more">')
  $trigger.append($menu)
  $(mainList).append($trigger)

  let isOpen = false
  let isSwiping = false
  let isScrolling = false
  let touchStartX = 0
  let touchStartY = 0
  let touchCurrentX = 0

  function isMobileLayout() {
    return window.innerWidth <= MOBILE_CUTOFF
  }

  function setTranslateX(x: number) {
    panel.style.transform = x ? `translateX(${x}px)` : ''
  }

  function open() {
    if (!isMobileLayout()) return
    isOpen = true
    $('html').addClass('slideout-open')
    setTranslateX(-PADDING)
    $hamburger.addClass('is-active')
    $overlay.show()
  }

  function close() {
    isOpen = false
    isSwiping = false
    isScrolling = false
    panel.style.transition = ''
    setTranslateX(0)
    $hamburger.removeClass('is-active')
    $overlay.hide()
    $('html').removeClass('slideout-open')
  }

  function handleNavbar() {
    let fromHide = false
    if (isMobileLayout()) {
      $menu.children().each(function () {
        const $element = $(this)
        $element.addClass('nav__list-item').removeClass('menu__item')
        $element.children('a').addClass('nav__item').removeClass('menu__link')
        $element.insertBefore($trigger, null)
      })
      $trigger.hide()
      return
    }
    $trigger.show()
    const base = 70
    let navItems = $('.nav__list--main > .nav__list-item')
    let navItem = navItems.length - 2
    while (navItem && $('#menu').children('div').height() > base) {
      if (!$(navItems[navItem]).children('a').hasClass('nav--active')) {
        $(navItems[navItem]).removeClass('nav__list-item').addClass('menu__item')
        $(navItems[navItem]).children('a').removeClass('nav__item').addClass('menu__link')
        $menu.prepend($(navItems[navItem]))
      }
      navItem--
      fromHide = true
    }
    if (!fromHide && $menu.children().length) {
      while ($('#menu').children('div').height() <= base && $menu.children().length) {
        const $element = $menu.children().first()
        $element.addClass('nav__list-item').removeClass('menu__item')
        $element.children('a').addClass('nav__item').removeClass('menu__link')
        $element.insertBefore($trigger, null)
      }
    }
    navItems = $('.nav__list--main > .nav__list-item')
    navItem = navItems.length - 2
    if ($('#menu').children('div').height() > base) {
      while (navItem && $('#menu').children('div').height() > base) {
        if (!$(navItems[navItem]).children('a').hasClass('nav--active')) {
          $(navItems[navItem]).removeClass('nav__list-item').addClass('menu__item')
          $(navItems[navItem]).children('a').removeClass('nav__item').addClass('menu__link')
          $menu.prepend($(navItems[navItem]))
        }
        navItem--
      }
    }
    if (!$menu.children().length) $trigger.hide()
  }

  const onLogoutClick = (event: Event) => {
    event.preventDefault()
    void options.onLogout()
  }
  $(document).on('click.hydroNavigation', '[name="nav_logout"]', onLogoutClick)
  const onOverlayClick = () => close()
  $overlay.on('click.hydroNavigation', onOverlayClick)
  const onHamburgerClick = () => (isOpen ? close() : open())
  $('.header__hamburger').on('click.hydroNavigation', onHamburgerClick)

  const onTouchStart = (event: TouchEvent) => {
    if (!isMobileLayout() || (event.target as Element).closest('[data-slideout-ignore]')) return
    touchStartX = event.touches[0].clientX
    touchStartY = event.touches[0].clientY
    touchCurrentX = touchStartX
    isSwiping = false
    isScrolling = false
  }
  const onTouchMove = (event: TouchEvent) => {
    if (!isMobileLayout()) return
    touchCurrentX = event.touches[0].clientX
    const dx = touchStartX - touchCurrentX
    if (!isSwiping) {
      if (isScrolling) return
      const dy = touchStartY - event.touches[0].clientY
      const absDx = Math.abs(dx)
      const absDy = Math.abs(dy)
      if (absDx < 10 && absDy < 10) return
      if (absDy > absDx) {
        isScrolling = true
        return
      }
      isSwiping = true
      if (!isOpen) $overlay.show()
      $('html').addClass('slideout-open')
      panel.style.transition = 'none'
    }
    setTranslateX(isOpen
      ? Math.max(-PADDING, Math.min(0, -PADDING - dx))
      : Math.max(-PADDING, Math.min(0, -dx)))
  }
  const onTouchEnd = () => {
    if (!isMobileLayout() || !isSwiping) return
    panel.style.transition = ''
    const dx = touchStartX - touchCurrentX
    if (isOpen) {
      if (-dx > TOLERANCE) close()
      else open()
    } else if (dx > TOLERANCE) open()
    else close()
    isSwiping = false
  }
  panel.addEventListener('touchstart', onTouchStart, { passive: true })
  panel.addEventListener('touchmove', onTouchMove, { passive: true })
  panel.addEventListener('touchend', onTouchEnd)

  const onResize = () => {
    handleNavbar()
    if (!isMobileLayout()) close()
  }
  window.addEventListener('resize', onResize)
  const interval = window.setInterval(handleNavbar, 3000)
  handleNavbar()

  return () => {
    window.clearInterval(interval)
    window.removeEventListener('resize', onResize)
    panel.removeEventListener('touchstart', onTouchStart)
    panel.removeEventListener('touchmove', onTouchMove)
    panel.removeEventListener('touchend', onTouchEnd)
    $(document).off('click.hydroNavigation', '[name="nav_logout"]', onLogoutClick)
    $overlay.off('click.hydroNavigation', onOverlayClick)
    $('.header__hamburger').off('click.hydroNavigation', onHamburgerClick)
    close()
    $trigger.remove()
  }
}
