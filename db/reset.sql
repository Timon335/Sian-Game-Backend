-- ล้างทุกตารางของ Sian Game เพื่อโหลด schema และ seed ใหม่ ข้อมูลทั้งหมดจะหายไป
DROP TABLE IF EXISTS reviews, wishlist_items, library_items, order_items, orders,
    cart_items, members, game_screenshots, game_genres, games, genres CASCADE;
