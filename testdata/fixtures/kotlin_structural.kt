package com.example

import org.junit.Test
import io.mockk.verify
import io.mockk.verifyOrder
import io.mockk.every
import io.mockk.slot

class StructuralTest {
    @Test
    fun `uses verifyOrder`() {
        verifyOrder {
            repo.load()
            repo.save(user)
        }
    }

    @Test
    fun `uses slot capture`() {
        val userSlot = slot<User>()
        every { repo.save(capture(userSlot)) } returns Unit
        sut.saveUser(testUser)
        assertEquals(testUser.name, userSlot.captured.name)
    }

    @Test
    fun `stubs and verifies same method`() {
        every { repo.save(any()) } returns Unit
        sut.process(data)
        verify { repo.save(any()) }
    }

    @Test
    fun `verify only - no output assertions`() {
        sut.process(data)
        verify { repo.save(any()) }
        verify { analytics.track(any()) }
    }

    @Test
    fun `normal behavioral test`() {
        every { repo.getUser() } returns testUser
        val result = sut.loadUser()
        assertThat(result).isEqualTo(testUser)
    }
}
