package com.example

import androidx.room.Room
import org.junit.Test
import org.junit.Before

class UserDaoRoomTest {

    private lateinit var db: AppDatabase

    @Before
    fun setUp() {
        db = Room.inMemoryDatabaseBuilder(context, AppDatabase::class.java).build()
    }

    @Test
    fun `inserts and retrieves user`() {
        val dao = db.userDao()
        dao.insert(testUser)
        val result = dao.getAll()
        assertThat(result).hasSize(1)
    }
}
